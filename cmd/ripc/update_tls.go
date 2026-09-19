package main

import (
	"bytes"
	"crypto"
	"crypto/sha256"
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

func (tlsUpdater) Update(tree *toml.Tree, path string) error {
	certificatePEM, ok := tree.Get("acme.certificate").(string)
	if !ok {
		return fmt.Errorf("%w: acme.certificate is not a string", ErrPathNotFound)
	}
	privateKeyPEM, ok := tree.Get("acme.private_key").(string)
	if !ok {
		return fmt.Errorf("%w: acme.private_key is not a string", ErrPathNotFound)
	}

	err := validateNonEmptyValue(certificatePEM, "acme.certificate")
	if err != nil {
		return err
	}
	err = validateNonEmptyValue(privateKeyPEM, "acme.private_key")
	if err != nil {
		return err
	}

	leaf, err := validateCertificate(certificatePEM)
	if err != nil {
		return err
	}

	privateKey, err := validatePrivateKey(privateKeyPEM)
	if err != nil {
		return err
	}

	err = validatePrivateKeyMatchesCertificate(leaf, privateKey)
	if err != nil {
		return err
	}

	err = validateCertificateFresh(leaf, time.Now())
	if err != nil {
		return err
	}

	tree.Set("server.tls.certificate", certificatePEM)
	tree.Set("server.tls.private_key", privateKeyPEM)
	return nil
}

// Print reports staged and live fingerprints with expiries. Fingerprints are full and never truncated; certificate chains and keys are never printed.
func (tlsUpdater) Print(ui UI, tree *toml.Tree, path string) error {
	certificatePEM, _ := tree.Get("acme.certificate").(string)
	privateKeyPEM, _ := tree.Get("acme.private_key").(string)

	leaf, err := validateCertificate(certificatePEM)
	if err != nil {
		return err
	}

	privateKey, err := validatePrivateKey(privateKeyPEM)
	if err != nil {
		return err
	}

	fingerprint, err := fingerprintPublicKey(leaf.PublicKey)
	if err != nil {
		return err
	}

	keyFingerprint, err := fingerprintPublicKey(privateKey.Public())
	if err != nil {
		return err
	}

	expiry := leaf.NotAfter.Format("2006-01-02")

	lines := []string{
		fmt.Sprintf("%-22s SHA256 %s (expires %s)", "acme.certificate", fingerprint, expiry),
		fmt.Sprintf("%-22s SHA256 %s (expires %s)", "server.tls.certificate", fingerprint, expiry),
		fmt.Sprintf("%-22s SHA256 %s", "acme.private_key", keyFingerprint),
		fmt.Sprintf("%-22s SHA256 %s", "server.tls.private_key", keyFingerprint),
	}

	for _, line := range lines {
		_, err := fmt.Fprintln(ui.Err, line)
		if err != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
		}
	}

	return nil
}

// fingerprintPublicKey returns the full SHA256 fingerprint of a public key in openssl colon form.
func fingerprintPublicKey(publicKey any) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("%w: failed to marshal public key: %w", ErrTLSKeyMismatch, err)
	}

	sum := sha256.Sum256(der)
	hexed := strings.ToUpper(hex.EncodeToString(sum[:]))

	groups := make([]string, 0, len(hexed)/2)
	for i := 0; i < len(hexed); i += 2 {
		groups = append(groups, hexed[i:i+2])
	}

	return strings.Join(groups, ":"), nil
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

// validatePrivateKey parses a PEM private key in EC, PKCS8 or PKCS1 form.
func validatePrivateKey(privateKeyPEM string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("%w: staged private key is not a PEM block", ErrTLSBadPrivateKey)
	}

	signer, err := validateECPrivateKey(block.Bytes)
	if err == nil {
		return signer, nil
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("%w: PKCS8 key does not implement crypto.Signer", ErrTLSBadPrivateKey)
		}
		return signer, nil
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse staged private key as EC, PKCS8 or PKCS1", ErrTLSBadPrivateKey)
	}

	return rsaKey, nil
}

// validateECPrivateKey parses DER bytes as an EC private key.
func validateECPrivateKey(der []byte) (crypto.Signer, error) {
	key, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// validatePrivateKeyMatchesCertificate reports whether privateKey is the key pair of the certificate. It compares the public keys in their DER form, the only comparison that is reliable across key types.
func validatePrivateKeyMatchesCertificate(leaf *x509.Certificate, privateKey crypto.Signer) error {
	certPublicKey, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal certificate public key: %w", ErrTLSKeyMismatch, err)
	}

	keyPublicKey, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return fmt.Errorf("%w: failed to marshal private key public part: %w", ErrTLSKeyMismatch, err)
	}

	if !bytes.Equal(certPublicKey, keyPublicKey) {
		return fmt.Errorf("%w: staged private key does not match the staged certificate", ErrTLSKeyMismatch)
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

// Errors returned by the TLS staged-to-live checks. A failure writes nothing.
var (
	ErrTLSEmptyStaged    = errors.New("staged value is empty")
	ErrTLSBadCertificate = errors.New("staged certificate is not usable")
	ErrTLSBadPrivateKey  = errors.New("staged private key is not usable")
	ErrTLSKeyMismatch    = errors.New("staged pair does not match")
	ErrTLSExpired        = errors.New("staged certificate is not fresh")
)
