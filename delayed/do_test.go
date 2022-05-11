package delayed

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

type mockDelayed struct {
	exec  int64
	index int
}

func (m mockDelayed) Do() {
	fmt.Println(m.index)
}

func (m mockDelayed) ExecTime() int64 {
	return m.exec
}

func (m mockDelayed) Identify() string {
	return "mock"
}

func nowMockDelayed(exec int64) Delayed {
	return mockDelayed{exec: exec}
}

func TestDispatchingDelayed_AddDelayed(t *testing.T) {

	type fields struct {
		taskList    []int64
		expectation []int64
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "顺序递增",
			fields: fields{
				taskList:    []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
		},
		{
			name: "顺序递减",
			fields: fields{
				taskList:    []int64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
		},
		{
			name: "随机",
			fields: fields{
				taskList:    []int64{5, 4, 3, 2, 1, 6, 7, 8, 9, 10},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
		},
		{
			name: "随机",
			fields: fields{
				taskList:    []int64{10, 9, 8, 7, 6, 5, 1, 2, 3, 4},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
		},

		{
			name: "顺序递增(奇数)",
			fields: fields{
				taskList:    []int64{1, 2, 3, 4, 5, 6, 7, 8, 9},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
		},
		{
			name: "顺序递减(奇数)",
			fields: fields{
				taskList:    []int64{9, 8, 7, 6, 5, 4, 3, 2, 1},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
		},
		{
			name: "随机(奇数)",
			fields: fields{
				taskList:    []int64{5, 4, 3, 2, 1, 6, 7, 8, 9},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
		},
		{
			name: "随机(奇数)",
			fields: fields{
				taskList:    []int64{9, 8, 7, 6, 5, 1, 2, 3, 4},
				expectation: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := DispatchingDelayed{}
			var cur []int64
			for _, task := range tt.fields.taskList {
				n.AddDelayed(nowMockDelayed(task))
			}
			for range n.delays {
				cur = append(cur, n.delDelayedTop().ExecTime())
			}
			assert.Equal(t, tt.fields.expectation, cur)
		})
	}
}

func TestDispatchingDelayed_AddDelayed2(t *testing.T) {

	n := NewDispatchingDelayed()
	for i := 0; i < 10; i++ {
		n.AddDelayed(mockDelayed{exec: time.Now().Add(time.Duration(i) * time.Second).Unix(), index: i})
	}
	// print 0...9
	time.Sleep(15 * time.Second)
}

func TestDispatchingDelayed_AddDelayed3(t *testing.T) {
	n := NewDispatchingDelayed(SetSingleCallback(func(signal os.Signal, d *DispatchingDelayed) {
		t.Log("signal")
	}))
	for i := 0; i < 10; i++ {
		n.AddDelayed(mockDelayed{exec: time.Now().Add(time.Duration(i) * time.Second).Unix(), index: i})
	}
	// print 0...9
	time.Sleep(time.Minute)
}
