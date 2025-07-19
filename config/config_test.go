package config

import (
	"log/slog"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestProvider_GetAndUpdate(t *testing.T) {
	t.Parallel()

	// Test that NewProvider panics with a nil config
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewProvider did not panic with nil config")
		}
	}()
	_ = NewProvider(nil)

	// Test Get and Update
	cfg1 := &Config{Server: Server{Addr: ":8080"}}
	provider := NewProvider(cfg1)
	if !reflect.DeepEqual(cfg1, provider.Get()) {
		t.Errorf("Get() got = %v, want %v", provider.Get(), cfg1)
	}

	cfg2 := &Config{Server: Server{Addr: ":9090"}}
	provider.Update(cfg2)
	if !reflect.DeepEqual(cfg2, provider.Get()) {
		t.Errorf("Get() got = %v, want %v", provider.Get(), cfg2)
	}
}

func TestProvider_Concurrency(t *testing.T) {
	t.Parallel()

	cfg1 := &Config{Server: Server{Addr: ":8080"}}
	cfg2 := &Config{Server: Server{Addr: ":9090"}}
	provider := NewProvider(cfg1)

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			// Alternate between reading and writing
			if i%2 == 0 {
				_ = provider.Get()
			} else {
				if i%4 == 1 {
					provider.Update(cfg2)
				} else {
					provider.Update(cfg1)
				}
			}
		}(i)
	}

	wg.Wait()

	// The final state is not deterministic, but this test is primarily for the race detector.
	// Running `go test -race` will fail if there are data races.
}

func TestDuration_UnmarshalText(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		input     string
		want      time.Duration
		expectErr bool
	}{
		{"Valid seconds", "10s", 10 * time.Second, false},
		{"Valid minutes", "5m", 5 * time.Minute, false},
		{"Invalid format", "bad", 0, true},
		{"Empty input", "", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tc.input))

			if (err != nil) != tc.expectErr {
				t.Fatalf("UnmarshalText() error = %v, expectErr %v", err, tc.expectErr)
			}
			if !tc.expectErr && d.Duration != tc.want {
				t.Errorf("UnmarshalText() got = %v, want %v", d.Duration, tc.want)
			}
		})
	}
}

func TestDuration_MarshalText(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		duration Duration
		want     string
	}{
		{"10 seconds", Duration{10 * time.Second}, "10s"},
		{"5 minutes", Duration{5 * time.Minute}, "5m0s"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.duration.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() returned an unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("MarshalText() got = %q, want %q", string(got), tc.want)
			}
		})
	}
}

func TestLogLevel_UnmarshalText(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		input     string
		want      slog.Level
		expectErr bool
	}{
		{"Lowercase info", "info", slog.LevelInfo, false},
		{"Uppercase debug", "DEBUG", slog.LevelDebug, false},
		{"Invalid level", "panic", 0, true},
		{"Empty input", "", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var l LogLevel
			err := l.UnmarshalText([]byte(tc.input))

			if (err != nil) != tc.expectErr {
				t.Fatalf("UnmarshalText() error = %v, expectErr %v", err, tc.expectErr)
			}
			if !tc.expectErr && l.Level != tc.want {
				t.Errorf("UnmarshalText() got = %v, want %v", l.Level, tc.want)
			}
		})
	}
}

func TestLogLevel_MarshalText(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name  string
		level LogLevel
		want  string
	}{
		{"Info level", LogLevel{slog.LevelInfo}, "INFO"},
		{"Debug level", LogLevel{slog.LevelDebug}, "DEBUG"},
		{"Warn level", LogLevel{slog.LevelWarn}, "WARN"},
		{"Error level", LogLevel{slog.LevelError}, "ERROR"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.level.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() returned an unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("MarshalText() got = %q, want %q", string(got), tc.want)
			}
		})
	}
}

func TestRegexp_UnmarshalText(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		input     string
		want      string // We check the string representation of the compiled regex
		expectErr bool
	}{
		{"Valid regex", "^test$", "^test$", false},
		{"Invalid regex", "^test(", "", true},
		{"Empty input gives nil regex", "", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var r Regexp
			err := r.UnmarshalText([]byte(tc.input))

			if (err != nil) != tc.expectErr {
				t.Fatalf("UnmarshalText() error = %v, expectErr %v", err, tc.expectErr)
			}
			if !tc.expectErr && r.String() != tc.want {
				t.Errorf("UnmarshalText() got = %v, want %v", r.String(), tc.want)
			}
		})
	}
}

func TestRegexp_MarshalText(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name  string
		regex *regexp.Regexp
		want  string
	}{
		{"Valid regex", regexp.MustCompile(`^test$`), `^test$`},
		{"Nil regex", nil, ``},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := Regexp{Regexp: tc.regex}
			got, err := r.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() returned an unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("MarshalText() got = %q, want %q", string(got), tc.want)
			}
		})
	}
}

func TestServer_BaseURL(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		server Server
		want   string
	}{
		{"HTTP", Server{Addr: "example.com:80", EnableTLS: false}, "http://example.com:80"},
		{"HTTPS", Server{Addr: "example.com:443", EnableTLS: true}, "https://example.com:443"},
		{"Empty host becomes localhost", Server{Addr: ":8080", EnableTLS: false}, "http://localhost:8080"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.server.BaseURL(); got != tc.want {
				t.Errorf("BaseURL() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEndpoints_Path(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"POST with space", "POST /api/login", "/api/login"},
		{"GET with space", "GET /api/users", "/api/users"},
		{"No method prefix", "/api/health", "/api/health"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var e Endpoints
			if got := e.Path(tc.endpoint); got != tc.want {
				t.Errorf("Path() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEndpoints_ConfirmHtml(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"Standard confirm", "POST /api/confirm-email", "/confirm-email.html"},
		{"No api prefix", "/confirm-password", "/confirm-password.html"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var e Endpoints
			if got := e.ConfirmHtml(tc.endpoint); got != tc.want {
				t.Errorf("ConfirmHtml() = %v, want %v", got, tc.want)
			}
		})
	}
}