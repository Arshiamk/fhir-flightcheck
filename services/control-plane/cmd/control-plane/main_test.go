package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	controlplane "github.com/Arshiamk/fhir-flightcheck/services/control-plane"
)

func TestLoopbackAddressDetection(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8080", "[::1]:8080"} {
		if !isLoopbackAddress(address) {
			t.Errorf("%q should be loopback", address)
		}
	}
	for _, address := range []string{":8080", "0.0.0.0:8080", "example.com:8080"} {
		if isLoopbackAddress(address) {
			t.Errorf("%q should not be loopback", address)
		}
	}
}

func TestDurableModeRequiresPersistentSigningKey(t *testing.T) {
	original := os.Getenv("FLIGHTCHECK_SIGNING_KEY")
	t.Cleanup(func() { _ = os.Setenv("FLIGHTCHECK_SIGNING_KEY", original) })
	_ = os.Unsetenv("FLIGHTCHECK_SIGNING_KEY")
	if _, err := loadSigner(true); err == nil {
		t.Fatal("durable mode accepted an ephemeral key")
	}
	if _, err := loadSigner(false); err != nil {
		t.Fatalf("local mode rejected ephemeral key: %v", err)
	}
}

func TestGenerateSigningKeyOutputRoundTrip(t *testing.T) {
	var output bytes.Buffer
	if err := generateSigningKey(&output); err != nil {
		t.Fatal(err)
	}
	var generated generatedSigningKey
	if err := json.Unmarshal(output.Bytes(), &generated); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, output.String())
	}
	if generated.Algorithm != "Ed25519" || generated.PrivateKey == "" ||
		generated.PublicKey == "" || generated.KeyID == "" {
		t.Fatalf("missing output fields: %+v", generated)
	}
	signer, err := controlplane.ParsePrivateKey(generated.PrivateKey)
	if err != nil {
		t.Fatalf("privateKey is not accepted by FLIGHTCHECK_SIGNING_KEY parser: %v", err)
	}
	if signer.PublicKeyBase64() != generated.PublicKey || signer.KeyID() != generated.KeyID {
		t.Fatal("privateKey round-trip did not reproduce publicKey and keyId")
	}
}

func TestNormalEphemeralWarningDoesNotLogPrivateKey(t *testing.T) {
	signer, err := controlplane.NewEphemeralSigner()
	if err != nil {
		t.Fatal(err)
	}
	privateKey := signer.PrivateKeyBase64()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	logEphemeralKeyWarning(logger, signer)
	if strings.Contains(logs.String(), privateKey) {
		t.Fatal("normal server warning logged the private key")
	}
	if !strings.Contains(logs.String(), signer.KeyID()) {
		t.Fatal("normal server warning omitted the non-secret key ID")
	}
}
