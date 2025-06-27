package config

import (
	"log/slog"
	"regexp"
	"time"

	"github.com/caasmo/restinpieces/crypto"
)

// NewDefaultConfig creates a new Config with sensible defaults.
// All secret values are randomly generated.
func NewDefaultConfig() *Config {
	return &Config{
		PublicDir: "static/dist",
		Jwt: Jwt{
			AuthSecret:                     crypto.RandomString(32, crypto.AlphanumericAlphabet),
			AuthTokenDuration:              Duration{Duration: 45 * time.Minute},
			VerificationEmailSecret:        crypto.RandomString(32, crypto.AlphanumericAlphabet),
			VerificationEmailTokenDuration: Duration{Duration: 24 * time.Hour},
			PasswordResetSecret:            crypto.RandomString(32, crypto.AlphanumericAlphabet),
			PasswordResetTokenDuration:     Duration{Duration: 1 * time.Hour},
			EmailChangeSecret:              crypto.RandomString(32, crypto.AlphanumericAlphabet),
			EmailChangeTokenDuration:       Duration{Duration: 1 * time.Hour},
		},
		Scheduler: Scheduler{
			Interval:              Duration{Duration: 60 * time.Second},
			MaxJobsPerTick:        10,
			ConcurrencyMultiplier: 2,
		},
		Log: Log{
			Request: LogRequest{
				Activated: true,
				Limits: LogRequestLimits{
					URILength:       512, // Minimum: 64
					UserAgentLength: 256, // Minimum: 32
					RefererLength:   512, // Minimum: 64
					RemoteIPLength:  64,  // Minimum: 15
				},
			},
			Batch: BatchLogger{
				FlushSize:     100,
				ChanSize:      1000,
				FlushInterval: Duration{Duration: 5 * time.Second},
				Level:         LogLevel{Level: slog.LevelInfo},
				DbPath:        "logs.db",
			},
		},
		Server: Server{
			Addr:                    ":8080",
			ShutdownGracefulTimeout: Duration{Duration: 15 * time.Second},
			ReadTimeout:             Duration{Duration: 2 * time.Second},
			ReadHeaderTimeout:       Duration{Duration: 2 * time.Second},
			WriteTimeout:            Duration{Duration: 3 * time.Second},
			IdleTimeout:             Duration{Duration: 1 * time.Minute},
			ClientIpProxyHeader:     "",
			EnableTLS:               false,
			CertData:                "",
			KeyData:                 "",
			RedirectAddr:            "",
		},
		RateLimits: RateLimits{
			PasswordResetCooldown:     Duration{Duration: 2 * time.Hour},
			EmailVerificationCooldown: Duration{Duration: 1 * time.Hour},
			EmailChangeCooldown:       Duration{Duration: 1 * time.Hour},
		},
		OAuth2Providers: map[string]OAuth2Provider{
			"google": {
				Name:            "google",
				DisplayName:     "Google",
				RedirectURL:     "",
				RedirectURLPath: "/oauth2/google/callback",
				AuthURL:         "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL:        "https://oauth2.googleapis.com/token",
				UserInfoURL:     "https://www.googleapis.com/oauth2/v3/userinfo",
				Scopes:          []string{"https://www.googleapis.com/auth/userinfo.profile", "https://www.googleapis.com/auth/userinfo.email"},
				PKCE:            true,
				ClientID:        "",
				ClientSecret:    "",
			},
		},
		Smtp: Smtp{
			Enabled:     false,
			Host:        "smtp.gmail.com",
			Port:        587,
			FromName:    "My App",
			FromAddress: "",
			LocalName:   "",
			AuthMethod:  "plain",
			UseTLS:      false,
			UseStartTLS: true,
			Username:    "",
			Password:    "",
		},
		Endpoints: Endpoints{
			RefreshAuth:              "POST /api/refresh-auth",
			RequestEmailVerification: "POST /api/request-email-verification",
			ConfirmEmailVerification: "POST /api/confirm-email-verification",
			ListEndpoints:            "GET /api/list-endpoints",
			AuthWithPassword:         "POST /api/auth-with-password",
			AuthWithOAuth2:           "POST /api/auth-with-oauth2",
			RegisterWithPassword:     "POST /api/register-with-password",
			ListOAuth2Providers:      "GET /api/list-oauth2-providers",
			RequestPasswordReset:     "POST /api/request-password-reset",
			ConfirmPasswordReset:     "POST /api/confirm-password-reset",
			RequestEmailChange:       "POST /api/request-email-change",
			ConfirmEmailChange:       "POST /api/confirm-email-change",
		},
		BlockIp: BlockIp{
			Enabled:   true,
			Activated: true,
		},
		Maintenance: Maintenance{
			Activated: false,
		},
		BlockUaList: BlockUaList{
			Activated: true,
			List: Regexp{
				Regexp: regexp.MustCompile(`(BotName\.v1|Super\-Bot|My\ Bot|AnotherBot)`),
			},
		},
		BlockRequestBody: BlockRequestBody{
			Activated: true,
			Limit:     1024 * 1024, // 1MB default limit
			ExcludedPaths: []string{
				"/api/upload",
				"/api/import",
			},
		},
		Notifier: Notifier{
			Discord: Discord{
				Activated:    false,
				WebhookURL:   "",
				APIRateLimit: Duration{Duration: 2 * time.Second},
				APIBurst:     1,
				SendTimeout:  Duration{Duration: 10 * time.Second},
			},
		},
		Metrics: Metrics{
			Enabled:    true,
			Activated:  true,
			Endpoint:   "/metrics",
			AllowedIPs: []string{"127.0.0.1", "::1"}, // Only exact IPs allowed, no CIDR ranges
		},
	}
}
