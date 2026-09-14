package task

import (
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math"
	"reflect"
	"testing"
)

func TestResultBSONRestoresConsumerTypes(t *testing.T) {
	for _, tt := range []struct {
		kind         string
		stored, want interface{}
	}{
		{"uint16", int32(65535), uint16(65535)},
		{"[]uint64", bson.A{int64(9007199254740993)}, []uint64{9007199254740993}},
		{"[]uint8", primitive.Binary{Data: []byte{0, 128, 255}}, []byte{0, 128, 255}},
		{"[]uint8", "AID/", []byte{0, 128, 255}}, // pre-fix durable JSON clone stored base64 strings
		{"[]uint8", nil, nil},
		{"unknown", int64(9007199254740993), int64(9007199254740993)},
		{"", int64(9007199254740993), int64(9007199254740993)},
	} {
		t.Run(fmt.Sprintf("%s-%T", tt.kind, tt.stored), func(t *testing.T) {
			body, err := bson.Marshal(bson.M{"type": tt.kind, "value": tt.stored})
			if err != nil {
				t.Fatal(err)
			}
			var got Result
			if err := bson.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Value, tt.want) {
				t.Fatalf("BSON value=%T(%v), want %T(%v)", got.Value, got.Value, tt.want, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		kind   string
		stored interface{}
	}{
		{"uint16", int32(-1)}, {"int8", int32(128)}, {"[]uint8", "%%%"}, {"[]uint8", primitive.Binary{Subtype: 4, Data: []byte{0}}},
	} {
		body, err := bson.Marshal(bson.M{"type": tt.kind, "value": tt.stored})
		if err != nil {
			t.Fatal(err)
		}
		var got Result
		if err := bson.Unmarshal(body, &got); err == nil {
			t.Errorf("accepted invalid BSON result type=%s value=%v", tt.kind, tt.stored)
		}
	}
}

func TestResultBSONFloat32Bounds(t *testing.T) {
	for _, tt := range []struct {
		kind         string
		stored, want interface{}
	}{
		{"float32", 0.1, float32(0.1)},
		{"float32", 3.4028235e38, float32(math.MaxFloat32)},
		{"float32", -3.4028235e38, -float32(math.MaxFloat32)},
		{"float32", math.Nextafter(float64(math.MaxFloat32), math.Inf(1)), float32(math.MaxFloat32)},
		{"float32", 1e-45, float32(math.SmallestNonzeroFloat32)},
		{"float32", math.Inf(1), float32(math.Inf(1))},
		{"[]float32", bson.A{0.1, 0.2, 3.4028235e38}, []float32{0.1, 0.2, math.MaxFloat32}},
		{"float64", 0.1, float64(0.1)},
	} {
		raw, err := bson.Marshal(bson.M{"type": tt.kind, "value": tt.stored})
		if err != nil {
			t.Fatal(err)
		}
		var got Result
		if err := bson.Unmarshal(raw, &got); err != nil {
			t.Errorf("%s(%v): %v", tt.kind, tt.stored, err)
			continue
		}
		if !reflect.DeepEqual(got.Value, tt.want) {
			t.Errorf("%s(%v) decoded %v want %v", tt.kind, tt.stored, got.Value, tt.want)
		}
	}
	for _, tt := range []struct {
		kind   string
		stored interface{}
	}{
		{"float32", 1e40}, {"float32", -1e40}, {"float32", math.NaN()}, {"[]float32", bson.A{0.1, 1e40}},
		{"int64", 0.1}, {"[]int32", bson.A{0.1, 0.2}}, {"uint64", 0.1},
	} {
		raw, err := bson.Marshal(bson.M{"type": tt.kind, "value": tt.stored})
		if err != nil {
			t.Fatal(err)
		}
		var got Result
		if err := bson.Unmarshal(raw, &got); err == nil {
			t.Errorf("accepted non-representable %s(%v)", tt.kind, tt.stored)
		}
	}
}
