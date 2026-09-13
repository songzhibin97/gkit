package zset_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/songzhibin97/gkit/structure/zset"
)

func TestRemoveRangeByRankBounds(t *testing.T) {
	for _, tt := range []struct {
		name                string
		length, start, stop int
		removed, remaining  string
	}{
		{"max start", 3, math.MaxInt, -1, "", "abc"},
		{"max stop", 3, 0, math.MaxInt, "abc", ""},
		{"both max", 3, math.MaxInt, math.MaxInt, "", "abc"},
		{"min start", 3, math.MinInt, -1, "abc", ""},
		{"min stop", 3, 0, math.MinInt, "", "abc"},
		{"both min", 3, math.MinInt, math.MinInt, "", "abc"},
		{"min to max", 3, math.MinInt, math.MaxInt, "abc", ""},
		{"negative tail", 3, -2, -1, "bc", "a"},
		{"negative start clamped", 3, -4, 1, "ab", "c"},
		{"negative stop outside", 3, 0, -4, "", "abc"},
		{"start outside", 3, 3, 4, "", "abc"},
		{"reversed", 3, 2, 1, "", "abc"},
		{"middle", 3, 1, 1, "b", "ac"},
		{"empty", 0, math.MinInt, math.MaxInt, "", ""},
		{"single max stop", 1, 0, math.MaxInt, "a", ""},
		{"single max start", 1, math.MaxInt, -1, "", "a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			z := zset.NewFloat64()
			for _, n := range rankBoundNodes("abc"[:tt.length]) {
				z.Add(n.Score, n.Value)
			}
			if got, want := z.RemoveRangeByRank(tt.start, tt.stop), rankBoundNodes(tt.removed); !reflect.DeepEqual(got, want) {
				t.Errorf("removed = %#v, want %#v", got, want)
			}
			if got, want := z.Range(0, -1), rankBoundNodes(tt.remaining); !reflect.DeepEqual(got, want) {
				t.Errorf("remaining = %#v, want %#v", got, want)
			}
			if z.Len() != len(tt.remaining) {
				t.Errorf("Len = %d, want %d", z.Len(), len(tt.remaining))
			}
			for _, n := range rankBoundNodes(tt.removed) {
				if score, ok := z.Score(n.Value); ok {
					t.Errorf("removed member %q still has score %v", n.Value, score)
				}
			}
			for i, n := range rankBoundNodes(tt.remaining) {
				if score, ok := z.Score(n.Value); !ok || score != n.Score {
					t.Errorf("Score(%q) = %v, %t", n.Value, score, ok)
				}
				if rank := z.Rank(n.Value); rank != i {
					t.Errorf("Rank(%q) = %d, want %d", n.Value, rank, i)
				}
			}
		})
	}
}

func rankBoundNodes(values string) []zset.Float64Node {
	var nodes []zset.Float64Node
	for _, value := range values {
		nodes = append(nodes, zset.Float64Node{Value: string(value), Score: float64(value - 'a' + 1)})
	}
	return nodes
}
