package config

import (
	"testing"
	"time"
)

func TestValidateLimiters(t *testing.T) {
	testCases := []struct {
		name     string
		limiters Limiter
		wantErr  bool
	}{
		{
			name: "valid",
			limiters: Limiter{
				"password_hashing": {Limit: Duration{Duration: 2 * time.Second}, Burst: 5},
			},
		},
		{
			name: "zero limit",
			limiters: Limiter{
				"password_hashing": {Limit: Duration{}, Burst: 5},
			},
			wantErr: true,
		},
		{
			name: "zero burst",
			limiters: Limiter{
				"password_hashing": {Limit: Duration{Duration: 2 * time.Second}, Burst: 0},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLimiters(tc.limiters)

			if tc.wantErr && err == nil {
				t.Fatal("validateLimiters() expected an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateLimiters() error = %v", err)
			}
		})
	}
}
