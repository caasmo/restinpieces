package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

func TestValidateEmail(t *testing.T) {
	testCases := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "test@sub.example.com", false},
		{"invalid email no at", "test.example.com", true},
		{"invalid email no domain", "test@", true},
		{"invalid email with spaces", "test @example.com", true},
		{"empty email", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEmail(tc.email)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	testCases := []struct {
		name                string
		remoteAddr          string
		proxyHeader         string
		proxyHeaderValue    string
		clientIpProxyHeader string
		expectedIP          string
	}{
		{
			name:       "direct connection ipv4",
			remoteAddr: "192.0.2.1:12345",
			expectedIP: "192.0.2.1",
		},
		{
			name:       "direct connection ipv6",
			remoteAddr: "[2001:db8::1]:12345",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "direct connection no port",
			remoteAddr: "192.0.2.1",
			expectedIP: "192.0.2.1",
		},
		{
			name:                "proxy connection",
			remoteAddr:          "198.51.100.1:54321",
			proxyHeader:         "X-Forwarded-For",
			proxyHeaderValue:    "203.0.113.1",
			clientIpProxyHeader: "X-Forwarded-For",
			expectedIP:          "203.0.113.1",
		},
		{
			name:                "proxy connection multiple ips",
			remoteAddr:          "198.51.100.1:54321",
			proxyHeader:         "X-Forwarded-For",
			proxyHeaderValue:    "203.0.113.1, 198.51.100.2",
			clientIpProxyHeader: "X-Forwarded-For",
			expectedIP:          "203.0.113.1",
		},
		{
			name:                "proxy header not set",
			remoteAddr:          "192.0.2.1:12345",
			clientIpProxyHeader: "X-Forwarded-For",
			expectedIP:          "192.0.2.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			cfg := &config.Config{
				Server: config.Server{ClientIpProxyHeader: tc.clientIpProxyHeader},
			}
			provider := &config.Provider{}
			provider.Update(cfg)

			app := &App{}
			app.SetConfigProvider(provider)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.proxyHeader != "" {
				req.Header.Set(tc.proxyHeader, tc.proxyHeaderValue)
			}

			// Execute
			ip := app.GetClientIP(req)

			// Verify
			if ip != tc.expectedIP {
				t.Errorf("GetClientIP() = %q, want %q", ip, tc.expectedIP)
			}
		})
	}
}
