package main

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml"
)

// tlsUpdater fills server.tls.certificate from the staged acme.certificate and server.tls.private_key from the staged acme.private_key.
type tlsUpdater struct{}

func (tlsUpdater) Update(tree *toml.Tree, arg string) error {
	certificatePEM, ok := tree.Get(tomlPathAcmeCertificate).(string)
	if !ok {
		return fmt.Errorf("%w: %s is not a string", ErrPathNotFound, tomlPathAcmeCertificate)
	}
	privateKeyPEM, ok := tree.Get(tomlPathAcmePrivateKey).(string)
	if !ok {
		return fmt.Errorf("%w: %s is not a string", ErrPathNotFound, tomlPathAcmePrivateKey)
	}

	err := validateNonEmptyValue(certificatePEM, tomlPathAcmeCertificate)
	if err != nil {
		return err
	}
	err = validateNonEmptyValue(privateKeyPEM, tomlPathAcmePrivateKey)
	if err != nil {
		return err
	}

	leaf, err := validateCertificate(certificatePEM)
	if err != nil {
		return err
	}

	err = validateKeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return err
	}

	err = validateCertificateFresh(leaf, time.Now())
	if err != nil {
		return err
	}

	err = validatePairDiffers(tree, certificatePEM, privateKeyPEM)
	if err != nil {
		return err
	}

	tree.Set(tomlPathServerTLSCertificate, certificatePEM)
	tree.Set(tomlPathServerTLSPrivateKey, privateKeyPEM)
	return nil
}

// Print reports the staged and live certificate fingerprints with their expiry. The fingerprint is the SHA256 of the certificate DER, matching `openssl x509 -fingerprint -sha256`; it is full and never truncated, and chains and keys are never printed.
func (tlsUpdater) Print(ui UI, tree *toml.Tree, arg string) error {
	certificatePEM, _ := tree.Get(tomlPathAcmeCertificate).(string)

	leaf, err := validateCertificate(certificatePEM)
	if err != nil {
		return err
	}

	fingerprint := fingerprintCertificate(leaf)
	expiry := leaf.NotAfter.Format("2006-01-02")

	lines := []string{
		fmt.Sprintf("%-22s SHA256 %s (expires %s)", tomlPathAcmeCertificate, fingerprint, expiry),
		fmt.Sprintf("%-22s SHA256 %s (expires %s)", tomlPathServerTLSCertificate, fingerprint, expiry),
	}

	for _, line := range lines {
		_, err := fmt.Fprintln(ui.Err, line)
		if err != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
		}
	}

	return nil
}

// fingerprintCertificate returns the full SHA256 fingerprint of the certificate DER in openssl colon form, matching `openssl x509 -fingerprint -sha256`.
func fingerprintCertificate(certificate *x509.Certificate) string {
	sum := sha256.Sum256(certificate.Raw)
	hexed := strings.ToUpper(hex.EncodeToString(sum[:]))

	groups := make([]string, 0, len(hexed)/2)
	for i := 0; i < len(hexed); i += 2 {
		groups = append(groups, hexed[i:i+2])
	}

	return strings.Join(groups, ":")
}

// validateNonEmptyValue reports an empty staged value under its path.
func validateNonEmptyValue(staged string, path string) error {
	if staged == "" {
		return fmt.Errorf("%w: staged value at %q is empty", ErrTLSEmptyStaged, path)
	}
	return nil
}

// validateCertificate parses the leaf certificate from the staged PEM chain.
func validateCertificate(certificatePEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certificatePEM))
	if block == nil {
		return nil, fmt.Errorf("%w: staged certificate is not a PEM block", ErrTLSBadCertificate)
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("%w: staged certificate PEM type is '%s', expected 'CERTIFICATE'", ErrTLSBadCertificate, block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse staged certificate: %w", ErrTLSBadCertificate, err)
	}

	return cert, nil
}

// validateKeyPair reports whether the staged private key parses and matches the staged certificate.
func validateKeyPair(certificatePEM string, privateKeyPEM string) error {
	_, err := tls.X509KeyPair([]byte(certificatePEM), []byte(privateKeyPEM))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTLSPairUnusable, err)
	}

	return nil
}

// validateCertificateFresh reports an expired staged certificate or one that is not valid yet.
func validateCertificateFresh(leaf *x509.Certificate, now time.Time) error {
	if !now.Before(leaf.NotAfter) {
		return fmt.Errorf("%w: staged certificate expired at %s", ErrTLSExpired, leaf.NotAfter.Format(time.RFC3339))
	}

	if now.Before(leaf.NotBefore) {
		return fmt.Errorf("%w: staged certificate is not valid before %s", ErrTLSExpired, leaf.NotBefore.Format(time.RFC3339))
	}

	return nil
}

// validatePairDiffers reports an already-current live pair, so the staged pair is not written again.
func validatePairDiffers(tree *toml.Tree, certificatePEM string, privateKeyPEM string) error {
	liveCertificate, _ := tree.Get(tomlPathServerTLSCertificate).(string)
	livePrivateKey, _ := tree.Get(tomlPathServerTLSPrivateKey).(string)
	if certificatePEM == liveCertificate && privateKeyPEM == livePrivateKey {
		return fmt.Errorf("%w: %s already matches the staged pair", ErrTLSSamePair, tomlPathServerTLS)
	}

	return nil
}

// Errors returned by the TLS staged-to-live checks. A failure writes nothing.
var (
	ErrTLSEmptyStaged    = errors.New("staged value is empty")
	ErrTLSBadCertificate = errors.New("staged certificate is not usable")
	ErrTLSPairUnusable   = errors.New("staged certificate and private key are not a usable pair")
	ErrTLSExpired        = errors.New("staged certificate is not fresh")
	ErrTLSSamePair       = errors.New("staged pair is already the live pair")
)
