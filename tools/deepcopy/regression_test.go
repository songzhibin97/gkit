package deepcopy

import (
	"math/big"
	"sync"
	"testing"
	"time"
)

type issue84Value struct {
	private int
	When    time.Time
	Pointer *int
	Slice   []string
	Skip    *int `gkit:"-"`
}

func TestClonePreservesValueStructAndDeepCopiesExportedReferences(t *testing.T) {
	pointer, skipped := 7, 11
	wantTime := time.Date(2024, time.March, 4, 5, 6, 7, 8, time.FixedZone("issue84", 8*60*60))
	src := issue84Value{
		private: 19,
		When:    wantTime,
		Pointer: &pointer,
		Slice:   []string{"before"},
		Skip:    &skipped,
	}

	got, ok := Clone(src).(issue84Value)
	if !ok {
		t.Fatalf("Clone returned %T, want issue84Value", Clone(src))
	}
	if got.private != src.private {
		t.Fatalf("private state = %d, want %d", got.private, src.private)
	}
	if !got.When.Equal(src.When) || got.When.Location() != src.When.Location() {
		t.Fatalf("time = %v (%p), want %v (%p)", got.When, got.When.Location(), src.When, src.When.Location())
	}
	if got.Pointer == nil || *got.Pointer != pointer {
		t.Fatalf("Pointer = %v, want pointer to %d", got.Pointer, pointer)
	}
	if got.Pointer == src.Pointer {
		t.Fatal("Pointer aliases the source")
	}
	if len(got.Slice) != 1 || got.Slice[0] != "before" {
		t.Fatalf("Slice = %v, want [before]", got.Slice)
	}
	if &got.Slice[0] == &src.Slice[0] {
		t.Fatal("Slice aliases the source")
	}
	if got.Skip != nil {
		t.Fatalf("gkit:- field = %v, want zero value", got.Skip)
	}

	*got.Pointer = 23
	got.Slice[0] = "after"
	if pointer != 7 || src.Slice[0] != "before" {
		t.Fatalf("mutating clone changed source: pointer=%d slice=%v", pointer, src.Slice)
	}
}

func TestDeepCopyPreservesTimeAndDestinationSkippedField(t *testing.T) {
	pointer, sourceSkipped, destinationSkipped := 7, 11, 13
	src := &issue84Value{
		private: 19,
		When:    time.Date(2024, time.March, 4, 5, 6, 7, 8, time.Local),
		Pointer: &pointer,
		Slice:   []string{"before"},
		Skip:    &sourceSkipped,
	}
	dst := issue84Value{Skip: &destinationSkipped}

	if err := DeepCopy(&dst, src); err != nil {
		t.Fatalf("DeepCopy: %v", err)
	}
	if dst.private != src.private || !dst.When.Equal(src.When) {
		t.Fatalf("value state not preserved: got private=%d time=%v, want private=%d time=%v", dst.private, dst.When, src.private, src.When)
	}
	if dst.Pointer == nil || dst.Pointer == src.Pointer || *dst.Pointer != pointer {
		t.Fatalf("Pointer = %v, source=%v", dst.Pointer, src.Pointer)
	}
	if len(dst.Slice) != 1 || &dst.Slice[0] == &src.Slice[0] {
		t.Fatalf("Slice = %v, source=%v", dst.Slice, src.Slice)
	}
	if dst.Skip != &destinationSkipped {
		t.Fatalf("gkit:- field was overwritten: got %v, want destination value", dst.Skip)
	}
}

func TestDeepCopyKeepsUnexportedSkippedField(t *testing.T) {
	type value struct {
		privateSkip int `gkit:"-"`
		Copied      int
	}
	src := &value{privateSkip: 7, Copied: 11}
	dst := value{privateSkip: 13}

	if err := DeepCopy(&dst, src); err != nil {
		t.Fatalf("DeepCopy: %v", err)
	}
	if dst.privateSkip != 13 || dst.Copied != 11 {
		t.Fatalf("DeepCopy = %+v, want privateSkip=13 Copied=11", dst)
	}
}

type issue84LockedValue struct {
	mu    sync.Mutex
	Value int
}

func TestCloneDoesNotCopyLockedMutexState(t *testing.T) {
	src := &issue84LockedValue{Value: 7}
	src.mu.Lock()
	defer src.mu.Unlock()

	got, ok := Clone(src).(*issue84LockedValue)
	if !ok {
		t.Fatalf("Clone returned %T, want *issue84LockedValue", Clone(src))
	}
	if got.Value != src.Value {
		t.Fatalf("Value = %d, want %d", got.Value, src.Value)
	}
	if !got.mu.TryLock() {
		t.Fatal("clone inherited the source mutex's locked state")
	}
	got.mu.Unlock()
}

func TestCloneDeepCopiesUnexportedBigIntStorage(t *testing.T) {
	src := new(big.Int).Lsh(big.NewInt(1), 256)
	want := new(big.Int).Set(src)

	got, ok := Clone(src).(*big.Int)
	if !ok {
		t.Fatalf("Clone returned %T, want *big.Int", Clone(src))
	}
	got.SetBit(got, 0, 1)

	if src.Cmp(want) != 0 {
		t.Fatalf("mutating clone changed source: got %v, want %v", src, want)
	}
	if got.Bit(0) != 1 {
		t.Fatalf("clone bit 0 = %d, want 1", got.Bit(0))
	}
}

type issue84LockedPrivateValue struct {
	mu         sync.Mutex
	private    int
	privateRef []int
}

func TestCloneResetsMutexAndPreservesPrivateState(t *testing.T) {
	src := &issue84LockedPrivateValue{private: 19, privateRef: []int{7}}
	src.mu.Lock()
	defer src.mu.Unlock()

	got, ok := Clone(src).(*issue84LockedPrivateValue)
	if !ok {
		t.Fatalf("Clone returned %T, want *issue84LockedPrivateValue", Clone(src))
	}
	if !got.mu.TryLock() {
		t.Fatal("clone inherited the source mutex's locked state")
	}
	got.mu.Unlock()
	if got.private != src.private {
		t.Fatalf("private = %d, want %d", got.private, src.private)
	}
	if len(got.privateRef) != 1 || got.privateRef[0] != 7 {
		t.Fatalf("privateRef = %v, want [7]", got.privateRef)
	}
	if &got.privateRef[0] == &src.privateRef[0] {
		t.Fatal("privateRef aliases the source")
	}
	got.privateRef[0] = 23
	if src.privateRef[0] != 7 {
		t.Fatalf("mutating clone changed source privateRef: %v", src.privateRef)
	}
}

func TestCloneNilReturnsNil(t *testing.T) {
	if got := Clone(nil); got != nil {
		t.Fatalf("Clone(nil) = %#v, want nil", got)
	}
}
