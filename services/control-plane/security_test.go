package controlplane

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestURLPolicyBlocksPrivateAddressesByDefault(t *testing.T) {
	policy := URLPolicy{AllowLocalDemo: true}
	for _, raw := range []string{
		"http://127.0.0.1:8080/fhir",
		"https://10.0.0.2/fhir",
		"https://[::1]/fhir",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := policy.Validate(context.Background(), raw, false); err == nil {
				t.Fatalf("Validate(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestURLPolicyRequiresBothLocalOptIns(t *testing.T) {
	raw := "http://127.0.0.1:8080/fhir"
	if _, err := (URLPolicy{}).Validate(context.Background(), raw, true); err == nil {
		t.Fatal("target opt-in bypassed server policy")
	}
	parsed, err := (URLPolicy{AllowLocalDemo: true}).Validate(context.Background(), raw, true)
	if err != nil {
		t.Fatalf("explicit local demo URL rejected: %v", err)
	}
	if parsed.String() != raw {
		t.Fatalf("got %q, want %q", parsed, raw)
	}
}

func TestURLPolicyRejectsCredentialsQueryAndFragments(t *testing.T) {
	policy := URLPolicy{AllowLocalDemo: true}
	for _, raw := range []string{
		"http://user:secret@127.0.0.1/fhir",
		"http://127.0.0.1/fhir?token=secret",
		"http://127.0.0.1/fhir#fragment",
	} {
		if _, err := policy.Validate(context.Background(), raw, true); err == nil {
			t.Errorf("Validate(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestRedact(t *testing.T) {
	value := Redact(`authorization: Bearer-secret token=abc123 password="hunter2"`)
	for _, secret := range []string{"Bearer-secret", "abc123", "hunter2"} {
		if strings.Contains(value, secret) {
			t.Errorf("redacted output retained %q: %s", secret, value)
		}
	}
}

// TestURLPolicyBlocksPrivateIPsReturnedByResolver simulates a DNS rebinding attack by
// directly calling validateIP with private addresses and confirming they are blocked.
func TestURLPolicyBlocksPrivateIPsReturnedByResolver(t *testing.T) {
	for _, ip := range []string{"192.168.1.1", "10.0.0.1", "172.16.0.1", "169.254.1.1"} {
		t.Run(ip, func(t *testing.T) {
			if err := validateIP(net.ParseIP(ip), false); err == nil {
				t.Fatalf("private IP %s not blocked", ip)
			}
		})
	}
}

// TestHTTPClientBlocksRedirectToPrivateAddress verifies that a 302 redirect leading to a
// private loopback address is blocked by the policy-aware HTTP client.
func TestHTTPClientBlocksRedirectToPrivateAddress(t *testing.T) {
	// Inner server: listens on loopback, returns 200.
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer inner.Close()

	// Outer server: redirects to the inner loopback server.
	outer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, inner.URL+"/internal", http.StatusFound)
	}))
	defer outer.Close()

	policy := URLPolicy{AllowLocalDemo: false}
	client := policy.HTTPClient(false, 5*time.Second)
	resp, err := client.Get(outer.URL) //nolint:noctx
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected redirect to private address to be blocked, but request succeeded")
	}
}

// TestHTTPClientReValidatesEachRedirectHop verifies that every hop in a redirect chain is
// re-validated, so a chain of safe redirects that eventually reaches a private address is
// still blocked.
func TestHTTPClientReValidatesEachRedirectHop(t *testing.T) {
	// Leaf server: listens on loopback, returns 200.
	leaf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer leaf.Close()

	// Middle server: redirects to leaf (loopback → loopback, both private).
	middle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, leaf.URL+"/step2", http.StatusFound)
	}))
	defer middle.Close()

	// Entry server: redirects to middle.
	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, middle.URL+"/step1", http.StatusFound)
	}))
	defer entry.Close()

	policy := URLPolicy{AllowLocalDemo: false}
	client := policy.HTTPClient(false, 5*time.Second)
	resp, err := client.Get(entry.URL) //nolint:noctx
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected multi-hop redirect ending at private address to be blocked")
	}
}
