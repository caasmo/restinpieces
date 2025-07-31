package mail

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
)

// mockSmtpServer is a lightweight, in-process SMTP server designed specifically
// for testing the mail package. It simulates a basic SMTP server that supports
// just enough of the protocol to allow our mailer to send an email, which is
// then captured for inspection.
//
// --- Key Behaviors & Limitations ---
//
// 1.  **No STARTTLS:** The server intentionally does NOT advertise or support the
//     STARTTLS command. In the EHLO response, it omits the "250-STARTTLS"
//     capability. This is crucial because it forces the client (mailyak) to
//     proceed with a plain, unencrypted connection, preventing the deadlocks
//     we encountered during development.
//
// 2.  **Plain Authentication Only:** It advertises and accepts only the "AUTH PLAIN"
//     mechanism. When the client sends this command, the server responds with a
//     standard success code ("235 Authentication Succeeded") without actually
//     validating any credentials.
//
// 3.  **Single Connection:** The server is designed to handle exactly one
//     client connection. This aligns with the testing strategy where each
//     test or sub-test creates its own isolated server instance, ensuring
//     a clean state for every test run.
//
// 4.  **Data Capture:** It captures all data sent after the "DATA" command and
//     stores it in the `data` field for assertions.
type mockSmtpServer struct {
	listener net.Listener
	addr     string
	data     string // Captured email data
	err      chan error
}

// newMockSmtpServer creates and starts a new mock SMTP server.
// It listens on a random available local port.
func newMockSmtpServer(t *testing.T) (*mockSmtpServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on a local port: %w", err)
	}

	server := &mockSmtpServer{
		listener: listener,
		addr:     listener.Addr().String(),
		err:      make(chan error, 1),
	}

	// Start the server loop in a background goroutine.
	go server.serve(t)

	return server, nil
}

// serve handles a single incoming client connection.
func (s *mockSmtpServer) serve(t *testing.T) {
	conn, err := s.listener.Accept()
	if err != nil {
		// If the listener was closed, just exit gracefully.
		if !strings.Contains(err.Error(), "use of closed network connection") {
			s.err <- err
		}
		return
	}
	// handleConnection will close the connection.
	s.handleConnection(t, conn)
}

// handleConnection processes a single client connection.
func (s *mockSmtpServer) handleConnection(t *testing.T, conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("error closing mock smtp server connection: %v", err)
		}
	}()

	reader := bufio.NewReader(conn)
	// Respond to the client's initial connection.
	if _, err := fmt.Fprint(conn, "220 mock-server ESMTP\r\n"); err != nil {
		return
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		// Log the received command for debugging
		t.Logf("mock-smtp-server received: %s", strings.TrimSpace(line))

		cmd := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case strings.HasPrefix(cmd, "HELO"):
			if _, err := fmt.Fprint(conn, "250 mock-server\r\n"); err != nil {
				return
			}
		case strings.HasPrefix(cmd, "EHLO"):
			if _, err := fmt.Fprint(conn, "250-mock-server\r\n"); err != nil {
				return
			}
			if _, err := fmt.Fprint(conn, "250 AUTH PLAIN\r\n"); err != nil {
				return
			}
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			if _, err := fmt.Fprint(conn, "235 2.7.0 Authentication Succeeded\r\n"); err != nil {
				return
			}
		case strings.HasPrefix(cmd, "MAIL FROM:"), strings.HasPrefix(cmd, "RCPT TO:"):
			if _, err := fmt.Fprint(conn, "250 OK\r\n"); err != nil {
				return
			}
		case strings.HasPrefix(cmd, "DATA"):
			if _, err := fmt.Fprint(conn, "354 End data with <CR><LF>.<CR><LF>\r\n"); err != nil {
				return
			}
			for {
				bodyLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if bodyLine == ".\r\n" {
					break
				}
				s.data += bodyLine
			}
			if _, err := fmt.Fprint(conn, "250 OK: queued as 12345\r\n"); err != nil {
				return
			}
		case strings.HasPrefix(cmd, "QUIT"):
			if _, err := fmt.Fprint(conn, "221 Bye\r\n"); err != nil {
				return
			}
			return
		}
	}
}

// Close stops the listener and cleans up the server.
func (s *mockSmtpServer) Close() {
	_ = s.listener.Close()
}

func setupTest(t *testing.T) (*mockSmtpServer, *Mailer, *config.Config) {
	t.Helper()

	server, err := newMockSmtpServer(t)
	if err != nil {
		t.Fatalf("Failed to start mock SMTP server: %v", err)
	}

	host, portStr, err := net.SplitHostPort(server.addr)
	if err != nil {
		t.Fatalf("Failed to parse mock server address: %v", err)
	}

	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	cfg := config.NewDefaultConfig()
	cfg.Smtp.Host = host
	cfg.Smtp.Port = port
	cfg.Smtp.FromName = "Test App"
	cfg.Smtp.FromAddress = "noreply@test.com"

	provider := config.NewProvider(cfg)

	mailer, err := New(provider)
	if err != nil {
		t.Fatalf("Failed to create mailer: %v", err)
	}

	return server, mailer, cfg
}

func TestSendVerificationEmail(t *testing.T) {
	server, mailer, cfg := setupTest(t)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := "test@example.com"
	callbackURL := "https://app.com/verify?token=123"
	err := mailer.SendVerificationEmail(ctx, email, callbackURL)

	if err != nil {
		t.Fatalf("SendVerificationEmail should not return an error, but got: %v", err)
	}

	select {
	case srvErr := <-server.err:
		t.Fatalf("Mock SMTP server encountered an error: %v", srvErr)
	default:
	}

	decodedData := decodeQuotedPrintable(t, server.data)
	assertContains(t, decodedData, fmt.Sprintf("To: %s", email))
	assertContains(t, decodedData, fmt.Sprintf("From: %s <%s>", cfg.Smtp.FromName, cfg.Smtp.FromAddress))
	assertContains(t, decodedData, fmt.Sprintf("Subject: Verify your %s email", cfg.Smtp.FromName))
	assertContains(t, decodedData, fmt.Sprintf(`href="%s"`, callbackURL))
}

func TestSendEmailChangeNotification(t *testing.T) {
	oldEmail := "old@example.com"
	newEmail := "new@example.com"
	callbackURL := "https://app.com/change?token=456"

	t.Run("with oauth2 login", func(t *testing.T) {
		server, mailer, cfg := setupTest(t)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := mailer.SendEmailChangeNotification(ctx, oldEmail, newEmail, true, callbackURL)
		if err != nil {
			t.Fatalf("SendEmailChangeNotification should not return an error, but got: %v", err)
		}

		decodedData := decodeQuotedPrintable(t, server.data)
		assertContains(t, decodedData, fmt.Sprintf("To: %s", newEmail))
		assertContains(t, decodedData, fmt.Sprintf("From: %s <%s>", cfg.Smtp.FromName, cfg.Smtp.FromAddress))
		assertContains(t, decodedData, fmt.Sprintf("Subject: Confirm your email change to %s", newEmail))
		assertContains(t, decodedData, "your old email is used for passwordless login")
		assertContains(t, decodedData, fmt.Sprintf(`href="%s"`, callbackURL))
	})

	t.Run("without oauth2 login", func(t *testing.T) {
		server, mailer, _ := setupTest(t)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := mailer.SendEmailChangeNotification(ctx, oldEmail, newEmail, false, callbackURL)
		if err != nil {
			t.Fatalf("SendEmailChangeNotification should not return an error, but got: %v", err)
		}

		decodedData := decodeQuotedPrintable(t, server.data)
		if strings.Contains(decodedData, "your old email is used for passwordless login") {
			t.Error("Email data should not contain the OAuth2 warning")
		}
		assertContains(t, decodedData, fmt.Sprintf(`href="%s"`, callbackURL))
	})
}

func TestSendPasswordResetEmail(t *testing.T) {
	server, mailer, cfg := setupTest(t)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := "reset@example.com"
	callbackURL := "https://app.com/reset?token=789"
	err := mailer.SendPasswordResetEmail(ctx, email, callbackURL)

	if err != nil {
		t.Fatalf("SendPasswordResetEmail should not return an error, but got: %v", err)
	}

	select {
	case srvErr := <-server.err:
		t.Fatalf("Mock SMTP server encountered an error: %v", srvErr)
	default:
	}

	decodedData := decodeQuotedPrintable(t, server.data)
	assertContains(t, decodedData, fmt.Sprintf("To: %s", email))
	assertContains(t, decodedData, fmt.Sprintf("From: %s <%s>", cfg.Smtp.FromName, cfg.Smtp.FromAddress))
	assertContains(t, decodedData, fmt.Sprintf("Subject: Reset your %s password", cfg.Smtp.FromName))
	assertContains(t, decodedData, fmt.Sprintf(`href="%s"`, callbackURL))
}

// assertContains is a helper function to check if a string contains a substring.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("Expected string to contain '%s', but it did not. Full string: %s", substr, s)
	}
}

func decodeQuotedPrintable(t *testing.T, s string) string {
	t.Helper()
	reader := strings.NewReader(s)
	qpReader := quotedprintable.NewReader(reader)
	decodedBytes, err := io.ReadAll(qpReader)
	if err != nil {
		t.Fatalf("Failed to decode quoted-printable: %v", err)
	}
	return string(decodedBytes)
}