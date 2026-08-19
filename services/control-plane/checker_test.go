package controlplane

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCapabilityCheckerOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{"valid R4", http.StatusOK, "application/fhir+json", `{"resourceType":"CapabilityStatement","fhirVersion":"4.0.1"}`, "pass"},
		{"wrong version", http.StatusOK, "application/fhir+json", `{"resourceType":"CapabilityStatement","fhirVersion":"5.0.0"}`, "fail"},
		{"wrong resource", http.StatusOK, "application/json", `{"resourceType":"Bundle","fhirVersion":"4.0.1"}`, "fail"},
		{"invalid JSON", http.StatusOK, "application/fhir+json", `{`, "fail"},
		{"HTTP failure", http.StatusServiceUnavailable, "application/fhir+json", `{}`, "fail"},
		{"wrong content type", http.StatusOK, "text/html", `{}`, "fail"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, _ = fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			checker := CapabilityChecker{
				Policy: URLPolicy{AllowLocalDemo: true}, Timeout: time.Second,
				Now: func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
			}
			result := checker.Check(t.Context(), Run{ID: "run_123"}, Target{
				BaseURL: server.URL, AllowPrivateNetwork: true,
			})
			if result.Finding.Outcome != test.want {
				t.Fatalf("outcome = %q, want %q (%s)", result.Finding.Outcome, test.want, result.Finding.Summary)
			}
			if len(result.Finding.EvidenceRefs) != 1 || len(result.Evidence.SHA256) != 64 {
				t.Fatalf("invalid evidence: %+v", result.Evidence)
			}
		})
	}
}

func TestCapabilityCheckerHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	checker := CapabilityChecker{
		Policy: URLPolicy{AllowLocalDemo: true}, Timeout: 10 * time.Millisecond,
	}
	result := checker.Check(t.Context(), Run{ID: "run_123"}, Target{
		BaseURL: server.URL, AllowPrivateNetwork: true,
	})
	if result.Finding.Outcome != "inconclusive" {
		t.Fatalf("outcome = %q, want inconclusive", result.Finding.Outcome)
	}
}
