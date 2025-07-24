package db

import (
	"testing"
	"time"
)

func TestTimeFormat(t *testing.T) {
	// Test case 1: A specific time
	loc, _ := time.LoadLocation("America/New_York")
	tt := time.Date(2024, 3, 11, 10, 4, 5, 0, loc) // 10:04:05 in EST is 14:04:05 in UTC
	expected := "2024-03-11T14:04:05Z"
	if got := TimeFormat(tt); got != expected {
		t.Errorf("TimeFormat() = %v, want %v", got, expected)
	}

	// Test case 2: Zero time
	var zeroTime time.Time
	expectedZero := "0001-01-01T00:00:00Z"
	if got := TimeFormat(zeroTime); got != expectedZero {
		t.Errorf("TimeFormat() for zero time = %v, want %v", got, expectedZero)
	}
}

func TestTimeParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "Valid RFC3339",
			input:   "2024-03-11T15:04:05Z",
			want:    time.Date(2024, 3, 11, 15, 4, 5, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Empty string",
			input:   "",
			want:    time.Time{},
			wantErr: false,
		},
		{
			name:    "Invalid format",
			input:   "2024-03-11 15:04:05",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "Valid RFC3339 with nanoseconds",
			input:   "2024-03-11T15:04:05.123456789Z",
			want:    time.Date(2024, 3, 11, 15, 4, 5, 123456789, time.UTC),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TimeParse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TimeParse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("TimeParse() = %v, want %v", got, tt.want)
			}
		})
	}
}