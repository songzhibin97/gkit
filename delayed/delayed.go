package delayed

type Delayed interface {
	Do()              // 执行任务
	ExecTime() int64  // 执行时间 time.Unix()
	Identify() string // 任务唯一标识
}

// copy https://github.dev/golang/go/blob/a131fd1313e0056ad094d234c67648409d081b8c/src/runtime/time.go siftupTimer and siftdownTimer

// Heap maintenance algorithms.
// These algorithms check for slice index errors manually.
// siftupDelayed puts the timer at position i in the right place
// in the heap by moving it up toward the top of the heap.
// It returns the smallest changed index.
func siftupDelayed(d []Delayed, i int) int {
	if i >= len(d) {
		return -1
	}
	when := d[i].ExecTime()
	if when <= 0 {
		return -1
	}
	tmp := d[i]
	for i > 0 {
		p := (i - 1) / 4
		if when >= d[p].ExecTime() {
			break
		}
		d[i] = d[p]
		i = p
	}
	if tmp != d[i] {
		d[i] = tmp
	}
	return i
}

// siftdownDelayed puts the Delayed at position i in the right place
// in the heap by moving it down toward the bottom of the heap.
func siftdownDelayed(d []Delayed, i int) {
	n := len(d)
	if i >= n {
		return
	}
	when := d[i].ExecTime()
	if when <= 0 {
		return
	}
	tmp := d[i]
	for {
		c := i*4 + 1 // left child
		c3 := c + 2  // mid child
		if c >= n {
			break
		}
		w := d[c].ExecTime()
		if c+1 < n && d[c+1].ExecTime() < w {
			w = d[c+1].ExecTime()
			c++
		}
		if c3 < n {
			w3 := d[c3].ExecTime()
			if c3+1 < n && d[c3+1].ExecTime() < w3 {
				w3 = d[c3+1].ExecTime()
				c3++
			}
			if w3 < w {
				w = w3
				c = c3
			}
		}
		if w >= when {
			break
		}
		d[i] = d[c]
		i = c
	}
	if tmp != d[i] {
		d[i] = tmp
	}
}
