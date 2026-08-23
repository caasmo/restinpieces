package config

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Validate checks the entire configuration for correctness.
// It aggregates validation checks from different parts of the configuration.
func Validate(cfg *Config) error {
	if err := validateServer(&cfg.Server); err != nil {
		return fmt.Errorf("server config validation failed: %w", err)
	}
	if err := validateJwt(&cfg.Jwt); err != nil {
		return fmt.Errorf("jwt config validation failed: %w", err)
	}
	if err := validateSmtp(&cfg.Smtp); err != nil {
		return fmt.Errorf("smtp config validation failed: %w", err)
	}
	// validateAcme call removed
	if err := validateOAuth2Providers(cfg.OAuth2Providers); err != nil {
		return fmt.Errorf("oauth2 providers validation failed: %w", err)
	}
	if err := validateBlockUaList(&cfg.BlockUaList); err != nil {
		return fmt.Errorf("block_ua_list config validation failed: %w", err)
	}
	if err := validateBlockHost(&cfg.BlockHost); err != nil {
		return fmt.Errorf("block_host config validation failed: %w", err)
	}
	if err := validateBlockOversizedRequest(&cfg.BlockOversizedRequest); err != nil {
		return fmt.Errorf("block_oversized_request config validation failed: %w", err)
	}
	if err := validateNotifier(&cfg.Notifier); err != nil {
		return fmt.Errorf("notifier config validation failed: %w", err)
	}
	if err := validateLoggerBatch(&cfg.Log.Batch); err != nil {
		return fmt.Errorf("logger_batch config validation failed: %w", err)
	}
	if err := validateRequestLog(&cfg.Log.Request); err != nil {
		return fmt.Errorf("request_log config validation failed: %w", err)
	}
	if err := validateBlockIp(&cfg.BlockIp); err != nil {
		return fmt.Errorf("block_ip config validation failed: %w", err)
	}
	if err := validateCache(&cfg.Cache); err != nil {
		return fmt.Errorf("cache config validation failed: %w", err)
	}
	if err := ValidateBackup(&cfg.Backup); err != nil {
		return fmt.Errorf("backup config validation failed: %w", err)
	}
	return nil
}

func validateBlockOversizedRequest(cfg *BlockOversizedRequest) error {
	if !cfg.Activated {
		return nil
	}

	if cfg.URLPathLimit < 0 {
		return fmt.Errorf("url_path_limit cannot be negative")
	}
	if cfg.QueryStringLimit < 0 {
		return fmt.Errorf("query_string_limit cannot be negative")
	}
	if cfg.HeaderCountLimit < 0 {
		return fmt.Errorf("header_count_limit cannot be negative")
	}
	if cfg.HeaderValueLimit < 0 {
		return fmt.Errorf("header_value_limit cannot be negative")
	}
	if cfg.BodyLimit < 0 {
		return fmt.Errorf("body_limit cannot be negative")
	}

	return nil
}

// isValidBackupLabel reports whether label is a valid backup map key.
//
// The label is the <key> in backup.online.<key>, backup.vacuum.<key>
// and backup.sqlite-rsync.entries.<key> and therefore part of every
// dot-path the tooling uses (ripc set/get/paths, TOML, origin wire
// label). It must not contain whitespace or '.'.
//
//   - whitespace (space, tab, newline) would require shell quoting and
//     would be marshaled as a quoted TOML key (e.g. [backup.online."my label"]),
//     breaking copy-pasteable `ripc set backup.online.<key>.source_path` commands.
//   - '.' would be split by the TOML tree as a nesting level
//     (backup.online.a.b → map entry a with sub-table b, not entry "a.b").
//
// Valid:   "app-online", "app-vacuum", "app-rsync", "app_db", "analytics".
// Invalid: "my label", "my.label", "", "app db", "a\tb".
func isValidBackupLabel(label string) bool {
	if label == "" {
		return false
	}
	if strings.ContainsAny(label, " \t\r\n.") {
		return false
	}
	return true
}

func ValidateBackup(backup *Backup) error {
	for key, e := range backup.OnlineAPI {
		if !isValidBackupLabel(key) {
			return fmt.Errorf("online: map key %q must not contain whitespace or '.'", key)
		}
		if err := validateBackupOnlineAPI(key, e); err != nil {
			return err
		}
	}
	for key, e := range backup.Vacuum {
		if !isValidBackupLabel(key) {
			return fmt.Errorf("vacuum: map key %q must not contain whitespace or '.'", key)
		}
		if err := validateBackupVacuum(key, e); err != nil {
			return err
		}
	}
	if len(backup.SqliteRsync.Entries) > 0 && backup.SqliteRsync.ListenAddr == "" {
		return fmt.Errorf("sqlite-rsync.listen_addr cannot be empty when entries are configured")
	}
	if backup.SqliteRsync.ListenAddr != "" {
		_, _, err := net.SplitHostPort(backup.SqliteRsync.ListenAddr)
		if err != nil {
			return fmt.Errorf("sqlite-rsync.listen_addr %q is not a valid host:port: %w", backup.SqliteRsync.ListenAddr, err)
		}
	}
	for key, e := range backup.SqliteRsync.Entries {
		if !isValidBackupLabel(key) {
			return fmt.Errorf("sqlite-rsync.entries: map key %q must not contain whitespace or '.'", key)
		}
		if err := validateBackupSqliteRsync(key, e); err != nil {
			return err
		}
	}
	return nil
}

func validateBackupOnlineAPI(key string, e BackupOnlineAPIEntry) error {
	if e.Frequency.Duration <= 0 {
		return fmt.Errorf("online.%s.frequency must be positive", key)
	}
	if e.SourcePath != "" && !isFile(e.SourcePath) {
		return fmt.Errorf("online.%s.source_path must be an existing file, got %q", key, e.SourcePath)
	}
	if e.DestPath != "" && !isDir(e.DestPath) {
		return fmt.Errorf("online.%s.dest_path must be an existing directory, got %q", key, e.DestPath)
	}
	if e.PagesPerStep <= 0 {
		return fmt.Errorf("online.%s.pages_per_step must be positive", key)
	}
	if e.SleepInterval.Duration < 0 {
		return fmt.Errorf("online.%s.sleep_interval cannot be negative", key)
	}
	return nil
}

func validateBackupVacuum(key string, e BackupVacuumEntry) error {
	if e.Frequency.Duration <= 0 {
		return fmt.Errorf("vacuum.%s.frequency must be positive", key)
	}
	if e.SourcePath != "" && !isFile(e.SourcePath) {
		return fmt.Errorf("vacuum.%s.source_path must be an existing file, got %q", key, e.SourcePath)
	}
	if e.DestPath != "" && !isDir(e.DestPath) {
		return fmt.Errorf("vacuum.%s.dest_path must be an existing directory, got %q", key, e.DestPath)
	}
	return nil
}

func validateBackupSqliteRsync(key string, e BackupSqliteRsyncEntry) error {
	if e.SourcePath != "" && !isFile(e.SourcePath) {
		return fmt.Errorf("sqlite-rsync.entries.%s.source_path must be an existing file, got %q", key, e.SourcePath)
	}
	if e.SyncTimeout.Duration < 0 {
		return fmt.Errorf("sqlite-rsync.entries.%s.sync_timeout cannot be negative", key)
	}
	return nil
}

// isDir reports whether path exists and is a directory. Relative
// paths resolve against the application CWD — validation runs inside the
// application, which knows its own CWD.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isFile reports whether path exists and is a regular file.
// Relative paths resolve against the application CWD.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// validateCache checks the Cache configuration section.
func validateCache(cache *Cache) error {
	if cache.Level == "" {
		return fmt.Errorf("cache.level cannot be empty")
	}

	// Validate that the level is one of the allowed values.
	allowedLevels := map[string]struct{}{
		"small":      {},
		"medium":     {},
		"large":      {},
		"very-large": {},
	}
	if _, ok := allowedLevels[cache.Level]; !ok {
		return fmt.Errorf("invalid cache.level '%s': must be one of 'small', 'medium', 'large', or 'very-large'", cache.Level)
	}

	return nil
}

// validateBlockIp checks the BlockIp configuration section.
func validateBlockIp(blockIp *BlockIp) error {
	if !blockIp.Enabled {
		return nil
	}

	if blockIp.Level == "" {
		return fmt.Errorf("block_ip.level cannot be empty")
	}

	// Validate that the level is one of the allowed values.
	allowedLevels := map[string]bool{"low": true, "medium": true, "high": true}
	if !allowedLevels[blockIp.Level] {
		return fmt.Errorf("invalid block_ip.level '%s': must be one of 'low', 'medium', or 'high'", blockIp.Level)
	}

	return nil
}

// validateLoggerBatch checks the batch logger configuration for logical consistency.
func validateLoggerBatch(loggerBatch *BatchLogger) error {
	if loggerBatch.ChanSize < 1 {
		return fmt.Errorf("chan_size must be >= 1")
	}
	if loggerBatch.FlushSize < 1 {
		return fmt.Errorf("flush_size must be >= 1")
	}
	if loggerBatch.FlushInterval.Duration <= 0 {
		return fmt.Errorf("flush_interval must be positive")
	}
	if loggerBatch.DbPath == "" {
		return fmt.Errorf("db_path cannot be empty")
	}
	// LogLevel validation is handled by UnmarshalText
	return nil
}

func validateRequestLog(requestLog *LogRequest) error {
	if !requestLog.Activated {
		return nil
	}

	minLimits := map[string]int{
		"url":        64,
		"user_agent": 32,
		"referer":    64,
		"remote_ip":  15, // Minimum for IPv4 (xxx.xxx.xxx.xxx)
	}

	if requestLog.Limits.URILength < minLimits["url"] {
		return fmt.Errorf("uri length limit must be at least %d", minLimits["url"])
	}
	if requestLog.Limits.UserAgentLength < minLimits["user_agent"] {
		return fmt.Errorf("user_agent length limit must be at least %d", minLimits["user_agent"])
	}
	if requestLog.Limits.RefererLength < minLimits["referer"] {
		return fmt.Errorf("referer length limit must be at least %d", minLimits["referer"])
	}
	if requestLog.Limits.RemoteIPLength < minLimits["remote_ip"] {
		return fmt.Errorf("remote_ip length limit must be at least %d", minLimits["remote_ip"])
	}

	return nil
}

func validateOAuth2Providers(providers map[string]OAuth2Provider) error {
	for name, provider := range providers {
		if provider.RedirectURL == "" && provider.RedirectURLPath == "" {
			return fmt.Errorf("oauth2 provider '%s' must have either RedirectURL or RedirectURLPath configured", name)
		}
		if provider.UserInfoURL != "" && !strings.HasPrefix(provider.UserInfoURL, "https://") {
			return fmt.Errorf("oauth2 provider '%s' UserInfoURL must use HTTPS: %s", name, provider.UserInfoURL)
		}
	}
	return nil
}

// validateServer checks the Server configuration section.
// It ensures the Addr field is not empty and contains a valid host:port or :port format.
// If only a port is provided (e.g., ":8080"), it defaults the host to "localhost".
//
// Allowed formats:
//   - "host:port" (e.g., "example.com:8080", "127.0.0.1:8080", "[::1]:8080")
//   - ":port"     (e.g., ":8080" becomes "localhost:8080")
//
// The port part is mandatory.
func validateServer(server *Server) error {
	if err := validateServerAddr(server); err != nil {
		return err
	}

	if err := validateServerRedirectAddr(server); err != nil {
		return err
	}

	if err := validateServerTLS(server); err != nil {
		return err
	}

	return nil
}

func sanitizeAddrEmptyHost(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

// validateServerAddr checks the Server.Addr field.
// It ensures the format is host:port or :port.
// Returns an error if the address is invalid.
func validateServerAddr(server *Server) error {
	if server.Addr == "" {
		return fmt.Errorf("server address cannot be empty")
	}

	// Split into host and port components
	_, port, err := net.SplitHostPort(server.Addr)
	if err != nil {
		return fmt.Errorf("invalid server address format '%s': %w", server.Addr, err)
	}

	// Validate the port component
	if err := validateServerPort(port); err != nil {
		return fmt.Errorf("invalid server port in address '%s': %w", server.Addr, err)
	}

	return nil
}

func validateServerRedirectAddr(server *Server) error {
	// If RedirectPort is empty, no redirect server is configured (valid case)
	if server.RedirectAddr == "" {
		return nil
	}

	// Construct the redirect address from the main server's host and redirect port
	_, port, err := net.SplitHostPort(server.RedirectAddr)
	if err != nil {
		return fmt.Errorf("failed to parse host from server address '%s': %w", server.Addr, err)
	}

	// Validate the port component
	if err := validateServerPort(port); err != nil {
		return fmt.Errorf("invalid server port in address '%s': %w", server.Addr, err)
	}

	return nil
}

// validateServerTLS checks that CertData and KeyData are present if TLS is enabled.
func validateServerTLS(server *Server) error {
	if !server.EnableTLS {
		return nil // No validation needed if TLS is disabled
	}

	// If TLS is enabled, CertData and KeyData must not be empty.
	// CertFile and KeyFile are ignored if CertData/KeyData are present.
	if server.CertData == "" {
		return fmt.Errorf("server.cert_data cannot be empty when TLS is enabled")
	}
	if server.KeyData == "" {
		return fmt.Errorf("server.key_data cannot be empty when TLS is enabled")
	}

	// Decode PEM block for the certificate
	block, _ := pem.Decode([]byte(server.CertData))
	if block == nil {
		return fmt.Errorf("server.cert_data: failed to decode PEM block containing the certificate")
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("server.cert_data: PEM block type is '%s', expected 'CERTIFICATE'", block.Type)
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("server.cert_data: failed to parse certificate: %w", err)
	}

	// Check certificate validity period
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("server.cert_data: certificate is not yet valid (valid from %s)", cert.NotBefore.Format(time.RFC3339))
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("server.cert_data: certificate has expired (expired on %s)", cert.NotAfter.Format(time.RFC3339))
	}

	// Optionally: Add more checks here, e.g., KeyUsage, BasicConstraints, etc.

	return nil
}

func validateJwt(jwt *Jwt) error {
	if jwt.AuthSecret == "" {
		return fmt.Errorf("jwt.auth_secret cannot be empty")
	}
	if jwt.PasswordResetSecret == "" {
		return fmt.Errorf("jwt.password_reset_secret cannot be empty")
	}
	if jwt.EmailChangeOtpSecret == "" {
		return fmt.Errorf("jwt.email_change_otp_secret cannot be empty")
	}
	if jwt.EmailChangeOtpTokenDuration.Duration <= 0 {
		return fmt.Errorf("jwt.email_change_otp_token_duration must be positive")
	}
	if jwt.VerificationEmailOtpSecret == "" {
		return fmt.Errorf("jwt.verification_email_otp_secret cannot be empty")
	}
	if jwt.VerificationEmailOtpTokenDuration.Duration <= 0 {
		return fmt.Errorf("jwt.verification_email_otp_token_duration must be positive")
	}
	if jwt.Oauth2StateSecret == "" {
		return fmt.Errorf("jwt.oauth2_state_secret cannot be empty")
	}
	return nil
}

func validateSmtp(smtp *Smtp) error {
	if !smtp.Enabled {
		return nil // No validation needed if SMTP is disabled
	}
	if smtp.Host == "" {
		return fmt.Errorf("smtp.host cannot be empty when enabled")
	}
	if smtp.Port == 0 {
		return fmt.Errorf("smtp.port cannot be 0 when enabled")
	}
	if smtp.FromAddress == "" {
		return fmt.Errorf("smtp.from_address cannot be empty when enabled")
	}
	if smtp.Username == "" {
		return fmt.Errorf("smtp.username cannot be empty when enabled")
	}
	if smtp.Password == "" {
		return fmt.Errorf("smtp.password cannot be empty when enabled")
	}
	return nil
}

// validateAcme function removed.

// validateBlockUaList checks the BlockUaList configuration section.
func validateBlockUaList(blockUaList *BlockUaList) error {
	if !blockUaList.Activated {
		return nil // No validation needed if UA blocking is disabled
	}

	// If activated, the regex must have been compiled successfully.
	// The Regexp field in our custom type will be nil if compilation failed
	// during UnmarshalText or if the input string was empty.
	if blockUaList.List.Regexp == nil {
		return fmt.Errorf("block_ua_list.list regex is invalid or empty, but blocking is activated")
	}

	return nil
}

func validateBlockHost(blockHost *BlockHost) error {
	if !blockHost.Activated {
		return nil
	}

	for _, host := range blockHost.AllowedHosts {
		if host == "" {
			return fmt.Errorf("block_host.allowed_hosts must not contain empty strings")
		}
		if strings.ContainsAny(host, " \t\r\n") {
			return fmt.Errorf("block_host.allowed_hosts: host '%s' contains whitespace characters", host)
		}
	}
	return nil
}

func validateNotifier(notifier *Notifier) error {

	if !notifier.Discord.Activated {
		return nil
	}

	if notifier.Discord.WebhookURL == "" {
		return fmt.Errorf("discord webhook_url cannot be empty when activated")
	}

	// Basic check for discord webhook domain
	if !strings.Contains(notifier.Discord.WebhookURL, "discord.com/api/webhooks/") &&
		!strings.Contains(notifier.Discord.WebhookURL, "discordapp.com/api/webhooks/") {
		return fmt.Errorf("discord webhook_url must contain discord.com/api/webhooks/ or discordapp.com/api/webhooks/")
	}

	return nil
}

func validateServerPort(portStr string) error {
	// Empty means no redirect server, which is valid configuration.
	if portStr == "" {
		return nil
	}

	// If set, it must be a valid port number
	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid RedirectPort '%s': must be a number: %w", portStr, err)
	}

	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("invalid RedirectPort '%d': port number must be between 1 and 65535", portNum)
	}

	return nil
}
