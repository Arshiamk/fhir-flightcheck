package controlplane

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadRuleCatalog(t *testing.T) {
	catalog, err := LoadRuleCatalog("../../packages/rule-packs")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Rules()) != 35 || len(catalog.Versions()) != 35 {
		t.Fatalf("loaded %d rules, want 35", len(catalog.Rules()))
	}
	if got := catalog.Capabilities(); !slices.Equal(got, []string{"fixtures", "network"}) {
		t.Fatalf("capabilities = %v", got)
	}
}

func TestRuleCatalogRejectsUnsafeCapabilities(t *testing.T) {
	directory := t.TempDir()
	body := `{"name":"unsafe","version":"1.0.0","rules":[{
		"schemaVersion":"1.0.0","id":"test.unsafe.write","version":"1.0.0",
		"title":"Unsafe rule","description":"This rule requests unsafe writes.",
		"category":"security","severity":"critical","behavior":"active-write",
		"deterministic":true,"capabilities":["write"],"timeoutSeconds":5,
		"evaluator":"unsafe_write","remediation":"Do not grant write capabilities."
	}]}`
	if err := os.WriteFile(filepath.Join(directory, "unsafe.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuleCatalog(directory); err == nil {
		t.Fatal("unsafe rule catalog was accepted")
	}
}
