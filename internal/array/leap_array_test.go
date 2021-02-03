package array

import (
	"Songzhibin/GKit/internal/clock"
	"Songzhibin/GKit/internal/sys/mutex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"reflect"
	"sync/atomic"
	"testing"
	"unsafe"
)

const (
	// BucketSize: 桶大小
	BucketSize uint64 = 500
	// N: 长度
	N uint64 = 20
	//IntervalSize: 时间间隔 10s
	IntervalSize uint64 = 10 * 1000
)

func Test_bucket_Size(t *testing.T) {
	b := &Bucket{
		Start: clock.GetTimeMillis(),
		Value: atomic.Value{},
	}
	if unsafe.Sizeof(*b) != 24 {
		t.Errorf("the size of BucketWrap is not equal 24.\n")
	}
	if unsafe.Sizeof(b) != 8 {
		t.Errorf("the size of BucketWrap pointer is not equal 8.\n")
	}
}

// mock ArrayMock and implement BucketGenerator
type leapArrayMock struct {
	mock.Mock
}

func (bla *leapArrayMock) NewEmptyBucket() interface{} {
	return new(int64)
}

func (bla *leapArrayMock) Reset(b *Bucket, startTime uint64) *Bucket {
	b.Start = startTime
	b.Value.Store(new(int64))
	return b
}

func Test_getTimeIndex(t *testing.T) {
	type fields struct {
		bucketSize   uint64
		n            uint64
		intervalSize uint64
		array        *AtomicArray
	}
	type args struct {
		timeMillis uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   uint64
	}{
		{
			name: "Test_getTimeIndex",
			fields: fields{
				bucketSize:   BucketSize,
				n:            N,
				intervalSize: IntervalSize,
				array:        NewAtomicArray(N, BucketSize, &leapArrayMock{}),
			},
			args: args{
				timeMillis: 1576296044907,
			},
			want: 9,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &LeapArray{
				bucketSize:   tt.fields.bucketSize,
				n:            tt.fields.n,
				intervalSize: tt.fields.intervalSize,
				array:        tt.fields.array,
				mu:           mutex.Mutex{},
			}
			if got := s.getTimeIndex(tt.args.timeMillis); got != tt.want {
				t.Errorf("getTimeIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_calculateStartTime(t *testing.T) {
	type fields struct {
	}
	type args struct {
		timeMillis uint64
		bucketSize uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   uint64
	}{
		{
			name:   "Test_calculateStartTime",
			fields: fields{},
			args: args{
				timeMillis: 1576296044907,
				bucketSize: BucketSize,
			},
			want: 1576296044500,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateStartTime(tt.args.timeMillis, tt.args.bucketSize); got != tt.want {
				t.Errorf("calculateStartTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getBucketOfTime(t *testing.T) {
	now := uint64(1596199310000)
	s := &LeapArray{
		bucketSize:   BucketSize,
		n:            N,
		intervalSize: IntervalSize,
		array:        NewAtomicArrayWithTime(N, BucketSize, now, &leapArrayMock{}),
		mu:           mutex.Mutex{},
	}
	got, err := s.getBucketOfTime(now+801, new(leapArrayMock))
	if err != nil {
		t.Errorf("getBucketOfTime() error = %v\n", err)
		return
	}
	if got.Start != now+500 {
		t.Errorf("BucketStart = %v, want %v", got.Start, now+500)
	}
	if !reflect.DeepEqual(got, s.array.getBucket(1)) {
		t.Errorf("getBucketOfTime() = %v, want %v", got, s.array.getBucket(1))
	}
}

func Test_getValueOfTime(t *testing.T) {
	type fields struct {
		bucketSize   uint64
		n            uint64
		intervalSize uint64
		array        *AtomicArray
	}
	type args struct {
		timeMillis uint64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Bucket
		wantErr bool
	}{
		{
			name: "Test_getValueOfTime",
			fields: fields{
				bucketSize:   BucketSize,
				n:            N,
				intervalSize: IntervalSize,
				array:        NewAtomicArrayWithTime(N, BucketSize, uint64(1596199310000), &leapArrayMock{}),
			},
			args: args{
				timeMillis: 1576296049907,
			},
			want:    nil,
			wantErr: false,
		},
	}
	// override start time
	start := uint64(1576296040000)
	for idx := (uint64)(0); idx < tests[0].fields.array.length; idx++ {
		ww := tests[0].fields.array.getBucket(idx)
		ww.Start = start
		start += 500
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &LeapArray{
				bucketSize:   tt.fields.bucketSize,
				n:            tt.fields.n,
				intervalSize: tt.fields.intervalSize,
				array:        tt.fields.array,
				mu:           mutex.Mutex{},
			}
			got := s.getValueOfTime(tt.args.timeMillis)
			for _, g := range got {
				find := false
				for i := (uint64)(0); i < tests[0].fields.array.length; i++ {
					w := tests[0].fields.array.getBucket(i)
					if w.Start == g.Start {
						find = true
						break
					}
				}
				if !find {
					t.Errorf("getValueOfTime() fail")
				}
			}
		})
	}
}

func Test_isDisable(t *testing.T) {
	type fields struct {
		bucketSize   uint64
		n            uint64
		intervalSize uint64
		array        *AtomicArray
	}
	type args struct {
		startTime uint64
		ww        *Bucket
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Test_isDisable",
			fields: fields{
				bucketSize:   BucketSize,
				n:            N,
				intervalSize: IntervalSize,
				array:        NewAtomicArrayWithTime(N, BucketSize, uint64(1596199310000), &leapArrayMock{}),
			},
			args: args{
				startTime: 1576296044907,
				ww: &Bucket{
					Start: 1576296004907,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			la := &LeapArray{
				bucketSize:   tt.fields.bucketSize,
				n:            tt.fields.n,
				intervalSize: tt.fields.intervalSize,
				array:        tt.fields.array,
				mu:           mutex.Mutex{},
			}
			if got := la.isDisable(tt.args.startTime, tt.args.ww); got != tt.want {
				t.Errorf("isDisable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewLeapArray(t *testing.T) {
	t.Run("TestNewLeapArray", func(t *testing.T) {
		_, err := NewLeapArray(N, IntervalSize, &leapArrayMock{})
		assert.Nil(t, err)
	})

	t.Run("TestNewLeapArray_nil", func(t *testing.T) {
		leapArray, err := NewLeapArray(N, IntervalSize, nil)
		assert.Nil(t, leapArray)
		assert.Error(t, err, ErrBucketBuilderIsNil)
	})

	t.Run("TestNewLeapArray_Invalid_Parameters", func(t *testing.T) {
		leapArray, err := NewLeapArray(30, IntervalSize, nil)
		assert.Nil(t, leapArray)
		assert.Error(t, err, ErrWindowNotSegmentation)
	})
}
