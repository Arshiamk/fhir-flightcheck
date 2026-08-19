package controlplane

import (
	"testing"
	"time"
)

func TestReportSigningRoundTripAndTamperDetection(t *testing.T) {
	signer, err := NewEphemeralSigner()
	if err != nil {
		t.Fatal(err)
	}
	report := Report{
		SchemaVersion: SchemaVersion, ReportID: "report_123", RunID: "run_123",
		ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Decision:       "ready", Coverage: Coverage{Selected: 1, Completed: 1},
		Findings: []Finding{}, CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := signer.Sign(&report); err != nil {
		t.Fatal(err)
	}
	public, err := ParsePublicKey(signer.PublicKeyBase64())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReport(report, public); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
	report.Decision = "not_ready"
	if err := VerifyReport(report, public); err == nil {
		t.Fatal("tampered report verified")
	}
}

func TestParseKeysRejectsInvalidLengths(t *testing.T) {
	if _, err := ParsePrivateKey("YQ=="); err == nil {
		t.Fatal("short private key accepted")
	}
	if _, err := ParsePublicKey("YQ=="); err == nil {
		t.Fatal("short public key accepted")
	}
}
