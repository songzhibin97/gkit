package registry

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"
)

func issue83Sorted(values []int) []int {
	result := append([]int(nil), values...)
	sort.Ints(result)
	return result
}

// Behavior 4: GetServices visits every row exactly once, including the only
// row for a single-client registry, and does not emit duplicate service IDs.
func TestIssue83RockSteadierVisitsEveryRow(t *testing.T) {
	tests := []struct {
		name     string
		clients  []int
		services []int
	}{
		{name: "single client", clients: []int{1}, services: []int{10, 11, 12}},
		{name: "last row", clients: []int{1, 2, 3}, services: []int{10, 11, 12}},
		{name: "deduplicate", clients: []int{1}, services: []int{10, 10, 11}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRockSteadierSubset(context.Background(), append([]int(nil), tt.clients...), tt.services, func() int64 { return 1 })
			defer r.Close()
			want := []int{10, 11, 12}
			if tt.name == "deduplicate" {
				want = []int{10, 11}
			}
			for _, client := range tt.clients {
				first := r.GetServices(client)
				got := issue83Sorted(first)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("GetServices(%d) = %v, want %v", client, got, want)
				}
				if again := r.GetServices(client); !reflect.DeepEqual(again, first) {
					t.Fatalf("GetServices(%d) changed from %v to %v without a registry update", client, first, again)
				}
			}
		})
	}
}

// Behavior 5: a non-positive requested subset has no members and never enters
// the division/shuffle path; positive-size selection remains intact.
func TestIssue83SubsetRejectsNonPositiveSize(t *testing.T) {
	instances := []interface{}{1, 2, 3, 4}
	for _, size := range []int{0, -1} {
		t.Run(string(rune('a'-size)), func(t *testing.T) {
			var got []interface{}
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("Subset(size=%d) panicked: %v", size, recovered)
					}
				}()
				got = Subset(instances, 3, size)
			}()
			if len(got) != 0 {
				t.Fatalf("Subset(size=%d) = %v, want empty", size, got)
			}
		})
	}
	if got := Subset(instances, 3, 2); len(got) != 2 {
		t.Fatalf("Subset(size=2) len = %d, want 2", len(got))
	}
}

// Behavior 6: every service owns one matrix position. Removal clears that
// position, and subsequent churn reuses it rather than growing the matrix.
func TestIssue83RockSteadierDeduplicatesAndReusesHoles(t *testing.T) {
	r := NewRockSteadierSubset(context.Background(), []int{1}, []int{10, 10}, func() int64 { return 1 })
	defer r.Close()

	matrix := r.matrixServices.Load().([][]*int)
	if len(matrix[0]) != 1 {
		t.Fatalf("initial row length = %d, want one unique slot", len(matrix[0]))
	}
	r.addService([]int{10, 10})
	matrix = r.matrixServices.Load().([][]*int)
	if len(matrix[0]) != 1 {
		t.Fatalf("row length after duplicate add = %d, want 1", len(matrix[0]))
	}

	r.removeService([]int{10})
	if got := r.GetServices(1); len(got) != 0 {
		t.Fatalf("services after remove = %v, want empty", got)
	}

	current := 20
	r.addService([]int{current})
	for next := 21; next < 121; next++ {
		r.removeService([]int{current})
		r.addService([]int{next, next})
		current = next
	}
	matrix = r.matrixServices.Load().([][]*int)
	if len(matrix[0]) != 1 {
		t.Fatalf("row length after churn = %d, want reused single slot", len(matrix[0]))
	}
	if got := r.GetServices(1); !reflect.DeepEqual(got, []int{current}) {
		t.Fatalf("services after churn = %v, want [%d]", got, current)
	}
}

func TestIssue83RockSteadierReusesHolesAcrossRows(t *testing.T) {
	r := NewRockSteadierSubset(context.Background(), []int{1, 2, 3}, []int{10, 11, 12}, func() int64 { return 1 })
	defer r.Close()

	const replacement = 99
	var removed int
	var hole [2]int
	for id, position := range r.hasService {
		if position[0] != r.appendIndex {
			removed = id
			hole = position
			break
		}
	}
	if removed == 0 {
		t.Fatal("test setup did not find a hole outside appendIndex")
	}
	originalSlots := 0
	for _, row := range r.matrixServices.Load().([][]*int) {
		originalSlots += len(row)
	}

	r.removeService([]int{removed})
	r.addService([]int{replacement})

	if got := r.hasService[replacement]; got != hole {
		t.Fatalf("replacement position = %v, want reused hole %v", got, hole)
	}
	currentSlots := 0
	for _, row := range r.matrixServices.Load().([][]*int) {
		currentSlots += len(row)
	}
	if currentSlots != originalSlots {
		t.Fatalf("matrix slots after cross-row reuse = %d, want %d", currentSlots, originalSlots)
	}
	for _, service := range r.GetServices(1) {
		if service == removed {
			t.Fatalf("removed service %d remained visible", removed)
		}
	}
}

func TestIssue83RockSteadierPublicLifecycleIsIdempotent(t *testing.T) {
	r := NewRockSteadierSubset(context.Background(), []int{1}, []int{10}, func() int64 { return 1 })
	defer r.Close()
	waitFor := func(want []int) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if got := issue83Sorted(r.GetServices(1)); reflect.DeepEqual(got, want) {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("GetServices(1) = %v, want %v", issue83Sorted(r.GetServices(1)), want)
	}

	if err := r.AddService(context.Background(), []int{10, 11, 11}); err != nil {
		t.Fatalf("AddService() error = %v", err)
	}
	waitFor([]int{10, 11})
	if err := r.RemoveService(context.Background(), []int{10}); err != nil {
		t.Fatalf("RemoveService() error = %v", err)
	}
	waitFor([]int{11})
	if err := r.AddService(context.Background(), []int{12, 12}); err != nil {
		t.Fatalf("AddService() reuse error = %v", err)
	}
	waitFor([]int{11, 12})

	matrix := r.matrixServices.Load().([][]*int)
	if len(matrix[0]) != 2 {
		t.Fatalf("row length after public lifecycle = %d, want reused two slots", len(matrix[0]))
	}
}
