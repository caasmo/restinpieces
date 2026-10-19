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
				TokenBucket: TokenBucket{
					"password_hashing": {Limit: Duration{Duration: 2 * time.Second}, Burst: 5},
				},
				FixedWindow: FixedWindow{
					"password_reset": {Prefix: "password_reset", Window: Duration{Duration: 2 * time.Hour}, Max: 1},
				},
			},
		},
		{
			name: "zero limit",
			limiters: Limiter{
				TokenBucket: TokenBucket{
					"password_hashing": {Limit: Duration{}, Burst: 5},
				},
			},
			wantErr: true,
		},
		{
			name: "zero burst",
			limiters: Limiter{
				TokenBucket: TokenBucket{
					"password_hashing": {Limit: Duration{Duration: 2 * time.Second}, Burst: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "fixed window zero window",
			limiters: Limiter{
				FixedWindow: FixedWindow{
					"password_reset": {Prefix: "password_reset", Window: Duration{}, Max: 1},
				},
			},
			wantErr: true,
		},
		{
			name: "fixed window zero max",
			limiters: Limiter{
				FixedWindow: FixedWindow{
					"password_reset": {Prefix: "password_reset", Window: Duration{Duration: 2 * time.Hour}, Max: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "fixed window empty prefix is rejected",
			limiters: Limiter{
				FixedWindow: FixedWindow{
					"upload": {Window: Duration{Duration: time.Hour}, Max: 1},
				},
			},
			wantErr: true,
		},
		{
			name: "label used by both branches",
			limiters: Limiter{
				TokenBucket: TokenBucket{
					"upload": {Limit: Duration{Duration: time.Second}, Burst: 1},
				},
				FixedWindow: FixedWindow{
					"upload": {Prefix: "upload", Window: Duration{Duration: time.Hour}, Max: 1},
				},
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
