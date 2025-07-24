package queue

import (
	"testing"
	"time"
)

func TestCoolDownBucket(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
		time     time.Time
		want     int
	}{
		{
			name:     "same bucket within duration",
			duration: time.Hour,
			time:     time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC),
			want:     473362,
		},
		{
			name:     "different bucket across duration",
			duration: time.Hour,
			time:     time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
			want:     473363,
		},
		{
			name:     "boundary condition just before next bucket",
			duration: time.Hour,
			time:     time.Date(2024, 1, 1, 10, 59, 59, 0, time.UTC),
			want:     473362,
		},
		{
			name:     "five minute duration",
			duration: 5 * time.Minute,
			time:     time.Date(2024, 1, 1, 10, 4, 0, 0, time.UTC),
			want:     5680344,
		},
		{
			name:     "epoch time",
			duration: time.Hour,
			time:     time.Unix(0, 0).UTC(),
			want:     0,
		},
		{
			name:     "start of next epoch bucket",
			duration: time.Hour,
			time:     time.Unix(3600, 0).UTC(),
			want:     1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoolDownBucket(tc.duration, tc.time); got != tc.want {
				t.Errorf("CoolDownBucket() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCoolDownBucket_Panic(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "zero duration",
			duration: 0,
		},
		{
			name:     "negative duration",
			duration: -1 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("CoolDownBucket() did not panic")
				}
			}()
			CoolDownBucket(tc.duration, time.Now())
		})
	}
}
