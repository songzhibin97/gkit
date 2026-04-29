package delayed

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"sync"
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

// TestIsInvalid 锁定 IsInvalid 必须基于哨兵指针单例 BadDelayed 比较；
// 早先 `delayed == badDelayed{}`（值类型）的写法对 *badDelayed 指针恒为 false。
func TestIsInvalid(t *testing.T) {
	d := &DispatchingDelayed{}
	assert.True(t, d.IsInvalid(BadDelayed))
	assert.False(t, d.IsInvalid(mockDelayed{exec: 1}))
}

// TestRefresh_NonBlocking 验证 Refresh 在 sentinel 未消费时不阻塞。
// refresh 通道 buffer=1，第一次写入后再次调用必须走 default 分支。
func TestRefresh_NonBlocking(t *testing.T) {
	d := &DispatchingDelayed{refresh: make(chan struct{}, 1)}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			d.Refresh()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Refresh blocked when channel buffer was full")
	}
}

// TestDelDelayed_LastIndex 覆盖 i == last 边界：
// 早先在末尾元素截断后访问 d.delays[last] 会触发 index out of range panic。
func TestDelDelayed_LastIndex(t *testing.T) {
	d := &DispatchingDelayed{}
	d.AddDelayed(mockDelayed{exec: 1})
	d.AddDelayed(mockDelayed{exec: 2})
	d.AddDelayed(mockDelayed{exec: 3})

	last := len(d.delays) - 1
	expected := d.delays[last]

	assert.NotPanics(t, func() {
		ret := d.delDelayed(last)
		assert.Equal(t, expected, ret)
	})
	assert.Equal(t, 2, len(d.delays))
}

// TestDelDelayed_OutOfRange 越界索引返回 BadDelayed，不修改堆。
func TestDelDelayed_OutOfRange(t *testing.T) {
	d := &DispatchingDelayed{}
	d.AddDelayed(mockDelayed{exec: 1})
	ret := d.delDelayed(10)
	assert.True(t, d.IsInvalid(ret))
	assert.Equal(t, 1, len(d.delays))
}

// TestGetTopDelayed_Race 在 -race 下应触发既往的数据竞争（修复前 RUnlock 立即调用，
// d.delays 读取无锁保护）。修复后 (defer d.RUnlock()) 应通过。
func TestGetTopDelayed_Race(t *testing.T) {
	d := &DispatchingDelayed{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(v int64) {
			defer wg.Done()
			d.AddDelayed(mockDelayed{exec: v})
		}(int64(i + 1))
		go func() {
			defer wg.Done()
			_ = d.getTopDelayed()
		}()
	}
	wg.Wait()
}

// TestSentinel_NoRaceOnDelays 在 -race 下覆盖 sentinel 主循环对 d.delays 的访问：
// 早先 `for i := 0; i < len(d.delays); i++` 与 close 流程的 `ln := len(d.delays)`
// 都是无锁读，与 AddDelayed 加锁写并发会触发 data race。
// 修复后 sentinel 改为 popIfReady（锁内 pop）/ pop 驱动循环，并发场景应通过。
func TestSentinel_NoRaceOnDelays(t *testing.T) {
	n := NewDispatchingDelayed(
		SetCheckTime(time.Millisecond),
		SetSingle(), // 关闭 signal handler，避免污染全局信号
	)
	defer func() { _ = n.Close() }()

	var wg sync.WaitGroup
	now := time.Now().Unix()
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(v int64) {
			defer wg.Done()
			n.AddDelayed(mockDelayed{exec: v})
			n.Refresh()
		}(now + int64(i%5))
	}
	wg.Wait()
	// 让 sentinel 跑几个 tick，触发与 AddDelayed 的窗口重叠
	time.Sleep(50 * time.Millisecond)
}
