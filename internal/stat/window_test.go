package stat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowResetWindow(t *testing.T) {
	window := NewWindow(3)
	for i := 0; i < 3; i++ {
		window.Append(i, 1.0)
	}
	window.ResetWindow()
	for i := 0; i < 3; i++ {
		assert.Equal(t, len(window.Bucket(i).Points), 0)
	}
}

func TestWindowResetBucket(t *testing.T) {
	window := NewWindow(3)
	for i := 0; i < 3; i++ {
		window.Append(i, 1.0)
	}
	window.ResetBucket(1)
	assert.Equal(t, len(window.Bucket(1).Points), 0)
	assert.Equal(t, window.Bucket(0).Points[0], 1.0)
	assert.Equal(t, window.Bucket(2).Points[0], 1.0)
}

func TestWindowResetBuckets(t *testing.T) {
	window := NewWindow(3)
	for i := 0; i < 3; i++ {
		window.Append(i, 1.0)
	}
	window.ResetBuckets([]int{0, 1, 2})
	for i := 0; i < 3; i++ {
		assert.Equal(t, len(window.Bucket(i).Points), 0)
	}
}

func TestWindowAppend(t *testing.T) {
	window := NewWindow(3)
	for i := 0; i < 3; i++ {
		window.Append(i, 1.0)
	}
	for i := 0; i < 3; i++ {
		assert.Equal(t, window.Bucket(i).Points[0], float64(1.0))
	}
}

func TestWindowAdd(t *testing.T) {
	window := NewWindow(3)
	window.Append(0, 1.0)
	window.Add(0, 1.0)
	assert.Equal(t, window.Bucket(0).Points[0], 2.0)
}

func TestWindowSize(t *testing.T) {
	window := NewWindow(3)
	assert.Equal(t, window.Size(), 3)
}
