package window

import (
	"reflect"
	"testing"

	"github.com/songzhibin97/gkit/internal/sys/mutex"
	"github.com/stretchr/testify/assert"
)

func TestGetTimeIndex(t *testing.T) {
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
			name: "TestGetTimeIndex",
			fields: fields{
				bucketSize:   BucketSize,
				n:            N,
				intervalSize: IntervalSize,
				array:        NewAtomicArray(N, BucketSize, &Mock{}),
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

func TestCalculateStartTime(t *testing.T) {
	type fields struct{}
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
			name:   "TestCalculateStartTime",
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

func TestGetBucketOfTime(t *testing.T) {
	now := uint64(1596199310000)
	s := &LeapArray{
		bucketSize:   BucketSize,
		n:            N,
		intervalSize: IntervalSize,
		array:        NewAtomicArrayWithTime(N, BucketSize, now, &Mock{}),
		mu:           mutex.Mutex{},
	}
	got, err := s.getBucketOfTime(now+801, new(Mock))
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

func TestGetValueOfTime(t *testing.T) {
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
			name: "TestGetValueOfTime",
			fields: fields{
				bucketSize:   BucketSize,
				n:            N,
				intervalSize: IntervalSize,
				array:        NewAtomicArrayWithTime(N, BucketSize, uint64(1596199310000), &Mock{}),
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

func TestIsDisable(t *testing.T) {
	type fields struct {
		bucketSize   uint64
		n            uint64
		intervalSize uint64
		array        *AtomicArray
	}
	type args struct {
		startTime uint64
		b         *Bucket
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "TestIsDisable",
			fields: fields{
				bucketSize:   BucketSize,
				n:            N,
				intervalSize: IntervalSize,
				array:        NewAtomicArrayWithTime(N, BucketSize, uint64(1596199310000), &Mock{}),
			},
			args: args{
				startTime: 1576296044907,
				b: &Bucket{
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
			if got := la.isDisable(tt.args.startTime, tt.args.b); got != tt.want {
				t.Errorf("isDisable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewLeapArray(t *testing.T) {
	t.Run("TestNewLeapArray", func(t *testing.T) {
		_, err := NewLeapArray(N, IntervalSize, &Mock{})
		assert.Nil(t, err)
	})

	t.Run("TestNewLeapArrayNil", func(t *testing.T) {
		leapArray, err := NewLeapArray(N, IntervalSize, nil)
		assert.Nil(t, leapArray)
		assert.Error(t, err, ErrBucketBuilderIsNil)
	})

	t.Run("TestNewLeapArrayInvalidParameters", func(t *testing.T) {
		leapArray, err := NewLeapArray(30, IntervalSize, nil)
		assert.Nil(t, leapArray)
		assert.Error(t, err, ErrWindowNotSegmentation)
	})
}
