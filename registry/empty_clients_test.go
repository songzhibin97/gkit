package registry

import (
	"context"
	"errors"
	"testing"
)

func TestRockSteadierSubsetHandlesEmptyClients(t *testing.T) {
	for _, test := range []struct {
		name     string
		services []int
	}{
		{name: "without services"},
		{name: "with services", services: []int{10, 11}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := NewRockSteadierSubset(context.Background(), nil, test.services, func() int64 { return 1 })
			defer r.Close()
			if got := r.GetServices(1); len(got) != 0 {
				t.Fatalf("GetServices with no clients = %v, want empty", got)
			}
			if err := r.AddService(context.Background(), []int{12}); !errors.Is(err, ErrorNoClients) {
				t.Errorf("AddService with no clients error = %v, want %v", err, ErrorNoClients)
			}
			if err := r.RemoveService(context.Background(), []int{10}); !errors.Is(err, ErrorNoClients) {
				t.Errorf("RemoveService with no clients error = %v, want %v", err, ErrorNoClients)
			}
		})
	}
}

func TestRockSteadierSubsetSelectionSurvivesEmptyServiceSet(t *testing.T) {
	r := NewRockSteadierSubset(context.Background(), []int{1}, []int{10}, func() int64 { return 1 })
	defer r.Close()
	if got := r.GetServices(1); len(got) != 1 || got[0] != 10 {
		t.Fatalf("initial GetServices(1) = %v, want [10]", got)
	}
	r.removeService([]int{10})
	if got := r.GetServices(1); len(got) != 0 {
		t.Fatalf("GetServices after removing all services = %v, want empty", got)
	}
	r.addService([]int{11})
	if got := r.GetServices(1); len(got) != 1 || got[0] != 11 {
		t.Fatalf("GetServices after restoring service = %v, want [11]", got)
	}
}
