package local_cache

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"reflect"
	"testing"
)

// Regression for #164, item 01-04: gob can encode a nil interface value, so
// Save must not reject it while registering the snapshot's concrete types.
func TestCacheSaveNilInterfaceRoundTrip(t *testing.T) {
	for _, test := range []struct {
		name   string
		member map[string]Iterator
	}{
		{name: "nil_only", member: map[string]Iterator{"nil": {Val: nil}}},
		{name: "mixed", member: map[string]Iterator{
			"nil":    {Val: nil},
			"string": {Val: "payload"},
			"int":    {Val: 42},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var standard bytes.Buffer
			if err := gob.NewEncoder(&standard).Encode(test.member); err != nil {
				t.Fatalf("standard gob encoding failed: %v", err)
			}
			assertNilSnapshotLoads(t, standard.Bytes(), test.member)

			source := NewCache()
			defer func() {
				if err := source.Shutdown(); err != nil {
					t.Error(err)
				}
			}()
			for key, item := range test.member {
				source.Set(key, item.Val, NoExpire)
			}
			var saved bytes.Buffer
			if err := source.Save(&saved); err != nil {
				t.Fatalf("Save of gob-encodable nil interface failed: %v", err)
			}
			var decoded map[string]Iterator
			if err := gob.NewDecoder(bytes.NewReader(saved.Bytes())).Decode(&decoded); err != nil {
				t.Fatalf("standard gob decode of Save output failed: %v", err)
			}
			if !reflect.DeepEqual(decoded, test.member) {
				t.Fatalf("saved snapshot = %#v, want %#v", decoded, test.member)
			}
			assertNilSnapshotLoads(t, saved.Bytes(), test.member)
		})
	}
}

func assertNilSnapshotLoads(t *testing.T, data []byte, want map[string]Iterator) {
	t.Helper()
	loaded := NewCache()
	defer func() {
		if err := loaded.Shutdown(); err != nil {
			t.Error(err)
		}
	}()
	if err := loaded.Load(bytes.NewReader(data)); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got := loaded.Count(); got != len(want) {
		t.Fatalf("loaded Count = %d, want %d", got, len(want))
	}
	for key, item := range want {
		if value, found := loaded.Get(key); !found || !reflect.DeepEqual(value, item.Val) {
			t.Fatalf("loaded Get(%q) = (%#v, %t), want (%#v, true)", key, value, found, item.Val)
		}
	}
	if value, found := loaded.Get("missing"); found || value != nil {
		t.Fatalf("missing Get = (%#v, %t), want (nil, false)", value, found)
	}
}

func TestCacheSaveNilInterfacePropagatesWriterError(t *testing.T) {
	c := NewCache()
	defer func() {
		if err := c.Shutdown(); err != nil {
			t.Error(err)
		}
	}()
	c.Set("nil", nil, NoExpire)
	reader, writer := io.Pipe()
	wantErr := errors.New("snapshot destination closed")
	if err := reader.CloseWithError(wantErr); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := c.Save(writer); err != wantErr {
		t.Fatalf("Save to closed pipe = %v, want %v", err, wantErr)
	}
	if value, found := c.Get("nil"); !found || value != nil {
		t.Fatalf("failed Save changed source Get(nil) = (%#v, %t), want (nil, true)", value, found)
	}
}
