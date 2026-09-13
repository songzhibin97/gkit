package page_token

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/encrypt/aes"
)

func TestTokenTimezoneProcess(t *testing.T) {
	mode := os.Getenv("GKIT_TOKEN_TZ_MODE")
	if mode == "" {
		return
	}
	pt := NewTokenGenerate("orders", SetSalt("timezone-test-salt"), SetTimeLimitation(time.Hour))
	path := os.Getenv("GKIT_TOKEN_TZ_FILE")
	switch mode {
	case "issue":
		encoded := pt.ForIndex(42)
		if encoded == "" {
			t.Fatal("empty generated token")
		}
		if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
			t.Fatal(err)
		}
	case "read":
		encoded, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		i, err := pt.GetIndex(string(encoded))
		if i != 42 || err != nil {
			t.Fatalf("index=%d error=%v", i, err)
		}
	case "expiry":
		TestTokenVersionedTimestampExpiry(t)
	case "legacy":
		tok := pt.(*token)
		for _, age := range []time.Duration{0, 2 * time.Hour} {
			encoded, err := aes.EncryptGCM(fmt.Sprintf("orders%s%s:42", resourceDelim, time.Now().Add(-age).Format(layout)), tok.salt)
			if err != nil {
				t.Fatal(err)
			}
			i, err := pt.GetIndex(encoded)
			if age == 0 {
				if i != 42 || err != nil {
					t.Fatalf("legacy fresh index=%d error=%v", i, err)
				}
			} else if !errors.Is(err, ErrOverdueToken) {
				t.Fatalf("legacy expired error=%v", err)
			}
		}
	default:
		t.Fatal("unknown fixture mode")
	}
}

func TestTokenCrossTimezoneRoundTrip(t *testing.T) {
	run := func(t *testing.T, zone, mode, path string) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestTokenTimezoneProcess$", "-test.timeout=10s")
		env := []string{}
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "TZ=") && !strings.HasPrefix(e, "GKIT_TOKEN_TZ_") {
				env = append(env, e)
			}
		}
		cmd.Env = append(env, "TZ="+zone, "GKIT_TOKEN_TZ_MODE="+mode, "GKIT_TOKEN_TZ_FILE="+path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("zone %s mode %s: %v\n%s", zone, mode, err, out)
		}
	}
	for _, zones := range [][2]string{{"UTC", "Asia/Shanghai"}, {"Asia/Shanghai", "UTC"}} {
		t.Run(zones[0]+"-to-"+zones[1], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "token")
			run(t, zones[0], "issue", path)
			run(t, zones[1], "read", path)
		})
	}
	for _, zone := range []string{"UTC", "Asia/Shanghai"} {
		t.Run("legacy-"+zone, func(t *testing.T) {
			run(t, zone, "legacy", "")
			run(t, zone, "expiry", "")
		})
	}
}

func TestTokenVersionedTimestampExpiry(t *testing.T) {
	pt := NewTokenGenerate("orders", SetSalt("timezone-test-salt"), SetTimeLimitation(time.Hour))
	tok := pt.(*token)
	for _, tc := range []struct {
		stamp string
		want  error
	}{
		{"v2|" + strconv.FormatInt(time.Now().UnixNano(), 10), nil},
		{"v2|" + strconv.FormatInt(time.Now().Add(-2*time.Hour).UnixNano(), 10), ErrOverdueToken},
		{"v2|bad", ErrInvalidToken}, {"v3|123", ErrInvalidToken},
	} {
		encoded, err := aes.EncryptGCM("orders"+resourceDelim+tc.stamp+":42", tok.salt)
		if err != nil {
			t.Fatal(err)
		}
		i, err := pt.GetIndex(encoded)
		if !errors.Is(err, tc.want) || (err == nil && i != 42) {
			t.Fatalf("timestamp case index=%d error=%v want=%v", i, err, tc.want)
		}
	}
}

func TestTokenVersionedTimestampWithoutTTL(t *testing.T) {
	pt := NewTokenGenerate("orders", SetSalt("timezone-test-salt"))
	tok := pt.(*token)
	encoded, err := aes.EncryptGCM("orders"+resourceDelim+"v2|"+strconv.FormatInt(time.Now().Add(-2*time.Hour).UnixNano(), 10)+":42", tok.salt)
	if err != nil {
		t.Fatal(err)
	}
	i, err := pt.GetIndex(encoded)
	if i != 42 || err != nil {
		t.Fatalf("without TTL index=%d error=%v", i, err)
	}
}
