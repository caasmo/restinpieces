package config

import (
	"log/slog"
	"regexp"
	"time"
)

// NewDefaultConfig creates a new Config with sensible defaults.
// Secret fields are empty — the caller must populate them before persisting.
//
// effective = defaults ← stored_overrides
// The key invariant is that defaults are always a complete, valid config. The
// stored TOML is intentionally partial — it only encodes intent to deviate.
//
// New fields in code always have a value, even if the stored config predates them
// Stale fields in the stored TOML are silently ignored on unmarshal
// No invalid config at startup
func NewDefaultConfig() *Config {
	return &Config{
		PublicDir: "static/dist",
		Jwt: Jwt{
			AuthSecret:                     "",
			AuthTokenDuration:              Duration{Duration: 45 * time.Minute},
			PasswordResetSecret:            "",
			PasswordResetTokenDuration:     Duration{Duration: 1 * time.Hour},
			EmailChangeOtpSecret:           "",
			EmailChangeOtpTokenDuration:    Duration{Duration: 15 * time.Minute},
			VerificationEmailOtpSecret:     "",
			VerificationEmailOtpTokenDuration: Duration{Duration: 15 * time.Minute},
			Oauth2StateSecret:              "",
			Oauth2StateTokenDuration:       Duration{Duration: 10 * time.Minute},
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
			EmailChangeCooldown:       Duration{Duration: 1 * time.Hour},
			EmailVerificationOtpCooldown: Duration{Duration: 2 * time.Minute},
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
			ListEndpoints:            "GET /api/list-endpoints",
			AuthWithPassword:         "POST /api/auth-with-password",
			AuthWithOAuth2:           "POST /api/auth-with-oauth2",
			RegisterWithPassword:     "POST /api/register-with-password",
			ListOAuth2Providers:      "GET /api/list-oauth2-providers",
			RequestPasswordResetOtp:  "POST /api/request-password-reset-otp",
			VerifyPasswordResetOtp:   "POST /api/verify-password-reset-otp",
			ConfirmPasswordResetOtp:  "POST /api/confirm-password-reset-otp",
			RequestEmailChangeOtp:  "POST /api/request-email-change-otp",
			ConfirmEmailChangeOtp:  "POST /api/confirm-email-change-otp",
			RequestEmailVerificationOtp: "POST /api/request-email-verification-otp",
			ConfirmEmailVerificationOtp: "POST /api/confirm-email-verification-otp",
		},
		BlockIp: BlockIp{
			Enabled:   true,
			Activated: true,
			Level:     "medium",
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
		BlockHost: BlockHost{
			Activated:    true,
			AllowedHosts: []string{},
		},
		BlockOversizedRequest: BlockOversizedRequest{
			Activated: true,
			URLPathLimit:     2048,
			QueryStringLimit: 2048,
			HeaderCountLimit: 100,
			HeaderValueLimit: 4096,
			BodyLimit:        1024 * 1024, // 1MB default limit
			ExcludedPaths: []string{
				"/api/upload",
				"/api/import",
			},
		},
		EndpointsBlockMismatch: EndpointsBlockMismatch{
			Activated: true,
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
		Backup: Backup{
			// Files is nil by default (the zero value). Entries are
			// created via ripc scaffold. Map keys are
			// user-chosen labels (not domain identifiers) — see AGENTS.md.
		},
		Cache: Cache{
			Level: "medium",
		},
	}
}

// NewBackupOnlineDefaults returns defaults for the Online Backup API strategy.
func NewBackupOnlineDefaults() BackupFile {
	return BackupFile{
		Strategy:               BackupStrategyOnline,
		Compression:            false,
		Frequency:              Duration{Duration: 15 * time.Minute},
		OnlineAPIPagesPerStep:  100,
		OnlineAPISleepInterval: Duration{Duration: 10 * time.Millisecond},
	}
}

// NewBackupVacuumDefaults returns defaults for the VACUUM INTO strategy.
func NewBackupVacuumDefaults() BackupFile {
	return BackupFile{
		Strategy:    BackupStrategyVacuum,
		Compression: false,
		Frequency:   Duration{Duration: 15 * time.Minute},
	}
}

// NewBackupSqliteRsyncDefaults returns defaults for the sqlite-rsync origin strategy.
// SyncTimeout is explicit 15m so the written TOML matches the daemon default.
func NewBackupSqliteRsyncDefaults() BackupFile {
	return BackupFile{
		Strategy:    BackupStrategySqliteRsync,
		SyncTimeout: Duration{Duration: 15 * time.Minute},
	}
}

// NewOAuth2ProviderDefaults returns an OAuth2Provider with sensible defaults
// for use by ripc scaffold. PKCE is enabled by default. Name,
// ClientID, ClientSecret, and URLs are empty — the user must configure them.
// Note: see TODO on OAuth2Providers in config.go regarding map key refactoring.
func NewOAuth2ProviderDefaults() OAuth2Provider {
	return OAuth2Provider{
		PKCE: true,
	}
}
