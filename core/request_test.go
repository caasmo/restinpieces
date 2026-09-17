package core

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

func TestClientIP(t *testing.T) {
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
		{
			name:       "direct connection ipv6 no port",
			remoteAddr: "2001:db8::1",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "direct connection expanded ipv6",
			remoteAddr: "[2001:0db8:0000:0000:0000:0000:0000:0001]:12345",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "direct connection ipv4 mapped to ipv6",
			remoteAddr: "[::ffff:192.0.2.1]:12345",
			expectedIP: "192.0.2.1",
		},
		{
			name:       "direct connection invalid address",
			remoteAddr: "invalid-address",
			expectedIP: "invalid-address",
		},
		{
			name:                "proxy connection ipv6",
			remoteAddr:          "198.51.100.1:54321",
			proxyHeader:         "CF-Connecting-IP",
			proxyHeaderValue:    "2001:0db8:0000:0000:0000:0000:0000:0001",
			clientIpProxyHeader: "CF-Connecting-IP",
			expectedIP:          "2001:db8::1",
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
			ip := app.ClientIP(req)

			// Verify
			if ip != tc.expectedIP {
				t.Errorf("GetClientIP() = %q, want %q", ip, tc.expectedIP)
			}
		})
	}
}

func TestClientUsesTLS(t *testing.T) {
	testCases := []struct {
		name                 string
		tlsConnection        bool
		proxyHeader          string
		proxyHeaderValue     string
		clientTLSProxyHeader string
		expected             bool
	}{
		{
			name:          "direct connection over tls",
			tlsConnection: true,
			expected:      true,
		},
		{
			name:     "direct connection without tls",
			expected: false,
		},
		{
			name:                 "proxy says https",
			proxyHeader:          "X-Forwarded-Proto",
			proxyHeaderValue:     "https",
			clientTLSProxyHeader: "X-Forwarded-Proto",
			expected:             true,
		},
		{
			name:                 "proxy says http",
			proxyHeader:          "X-Forwarded-Proto",
			proxyHeaderValue:     "http",
			clientTLSProxyHeader: "X-Forwarded-Proto",
			expected:             false,
		},
		{
			name:                 "proxy says https in upper case",
			proxyHeader:          "X-Forwarded-Proto",
			proxyHeaderValue:     "HTTPS",
			clientTLSProxyHeader: "X-Forwarded-Proto",
			expected:             true,
		},
		{
			name:                 "proxy sends several values",
			proxyHeader:          "X-Forwarded-Proto",
			proxyHeaderValue:     "https, http",
			clientTLSProxyHeader: "X-Forwarded-Proto",
			expected:             true,
		},
		{
			name:                 "proxy header not set",
			proxyHeader:          "X-Forwarded-Proto",
			clientTLSProxyHeader: "X-Forwarded-Proto",
			expected:             false,
		},
		{
			name:             "proxy header not configured",
			proxyHeader:      "X-Forwarded-Proto",
			proxyHeaderValue: "https",
			expected:         false,
		},
		{
			name:                 "tls connection wins over proxy http",
			tlsConnection:        true,
			proxyHeader:          "X-Forwarded-Proto",
			proxyHeaderValue:     "http",
			clientTLSProxyHeader: "X-Forwarded-Proto",
			expected:             true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			cfg := &config.Config{
				Server: config.Server{ClientTLSProxyHeader: tc.clientTLSProxyHeader},
			}
			provider := &config.Provider{}
			provider.Update(cfg)

			app := &App{}
			app.SetConfigProvider(provider)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.tlsConnection {
				req.TLS = &tls.ConnectionState{}
			}
			if tc.proxyHeader != "" {
				req.Header.Set(tc.proxyHeader, tc.proxyHeaderValue)
			}

			// Execute
			usesTLS := app.ClientUsesTLS(req)

			// Verify
			if usesTLS != tc.expected {
				t.Errorf("ClientUsesTLS() = %v, want %v", usesTLS, tc.expected)
			}
		})
	}
}
