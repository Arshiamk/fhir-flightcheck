package controlplane

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type Signer struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	keyID   string
}

func NewEphemeralSigner() (*Signer, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	return NewSigner(private)
}

func NewSigner(private ed25519.PrivateKey) (*Signer, error) {
	if len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Ed25519 private key length")
	}
	public := private.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(public)
	return &Signer{private: private, public: public, keyID: "ed25519:" + hex.EncodeToString(sum[:8])}, nil
}

func ParsePrivateKey(value string) (*Signer, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode signing key: %w", err)
	}
	return NewSigner(ed25519.PrivateKey(raw))
}

func (s *Signer) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(s.public)
}

// PrivateKeyBase64 returns the PKCS-independent 64-byte Ed25519 private key
// encoding accepted by FLIGHTCHECK_SIGNING_KEY.
func (s *Signer) PrivateKeyBase64() string {
	return base64.StdEncoding.EncodeToString(s.private)
}

func (s *Signer) KeyID() string { return s.keyID }

func reportPayload(report Report) ([]byte, error) {
	report.Signature = nil
	return json.Marshal(report)
}

func (s *Signer) Sign(report *Report) error {
	payload, err := reportPayload(*report)
	if err != nil {
		return fmt.Errorf("marshal report for signing: %w", err)
	}
	report.Signature = &Signature{
		Algorithm: "Ed25519",
		KeyID:     s.keyID,
		Value:     base64.StdEncoding.EncodeToString(ed25519.Sign(s.private, payload)),
	}
	return nil
}

func ParsePublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("invalid Ed25519 public key length")
	}
	return ed25519.PublicKey(raw), nil
}

func VerifyReport(report Report, public ed25519.PublicKey) error {
	if report.Signature == nil {
		return errors.New("report is unsigned")
	}
	if report.Signature.Algorithm != "Ed25519" {
		return fmt.Errorf("unsupported signature algorithm %q", report.Signature.Algorithm)
	}
	signature, err := base64.StdEncoding.DecodeString(report.Signature.Value)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	payload, err := reportPayload(report)
	if err != nil {
		return fmt.Errorf("marshal report for verification: %w", err)
	}
	if !ed25519.Verify(public, payload, signature) {
		return errors.New("report signature verification failed")
	}
	sum := sha256.Sum256(public)
	expectedKeyID := "ed25519:" + hex.EncodeToString(sum[:8])
	if report.Signature.KeyID != expectedKeyID {
		return errors.New("report key ID does not match public key")
	}
	return nil
}
