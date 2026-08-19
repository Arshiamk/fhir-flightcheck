package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	controlplane "github.com/Arshiamk/fhir-flightcheck/services/control-plane"
)

const (
	exitOK           = 0
	exitGateFailed   = 1
	exitUsage        = 2
	exitServiceError = 3
	exitVerifyFailed = 4
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitUsage
	}
	ctx := context.Background()
	switch args[0] {
	case "init":
		return commandInit(ctx, args[1:], stdout, stderr)
	case "target":
		if len(args) > 1 && args[1] == "add" {
			return commandTargetAdd(ctx, args[2:], stdout, stderr)
		}
	case "run":
		return commandRun(ctx, args[1:], stdout, stderr)
	case "report":
		if len(args) > 1 && args[1] == "verify" {
			return commandReportVerify(args[2:], stdout, stderr)
		}
	case "baseline":
		if len(args) > 1 && args[1] == "set" {
			return commandBaselineSet(ctx, args[2:], stdout, stderr)
		}
	case "ci":
		return commandCI(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return exitOK
	}
	fmt.Fprintln(stderr, "unknown or incomplete command")
	printUsage(stderr)
	return exitUsage
}

func commandInit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("init", stderr)
	apiURL := flags.String("api", "http://127.0.0.1:8080", "control plane URL")
	apiToken := flags.String("token", os.Getenv("FLIGHTCHECK_API_TOKEN"), "API bearer token (prefer FLIGHTCHECK_API_TOKEN)")
	name := flags.String("name", "Local Flightcheck", "project name")
	configPath := flags.String("config", defaultConfigPath, "configuration path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return exitUsage
	}
	client, err := newAPIClient(*apiURL)
	if err != nil {
		return reportError(stderr, exitUsage, err)
	}
	client.token = *apiToken
	var project controlplane.Project
	if err := client.mutate(ctx, http.MethodPost, "/v1/projects", idempotencyKey(), map[string]string{"name": *name}, &project); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	var key struct {
		KeyID     string `json:"keyId"`
		PublicKey string `json:"publicKey"`
	}
	if err := client.get(ctx, "/v1/signing-key", &key); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	value := config{APIURL: client.baseURL, ProjectID: project.ID, PublicKey: key.PublicKey, KeyID: key.KeyID}
	if err := saveConfig(*configPath, value); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	fmt.Fprintf(stdout, "Initialized project %s in %s\n", project.ID, *configPath)
	return exitOK
}

func commandTargetAdd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("target add", stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration path")
	name := flags.String("name", "", "target name")
	baseURL := flags.String("url", "", "FHIR base URL")
	credentialRef := flags.String("credential-ref", "none", "credential reference (never a secret)")
	allowLocal := flags.Bool("allow-local-demo", false, "explicitly allow a private/local demo target")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *name == "" || *baseURL == "" {
		fmt.Fprintln(stderr, "target add requires --name and --url")
		return exitUsage
	}
	value, client, code := configuredClient(*configPath, stderr)
	if code != exitOK {
		return code
	}
	input := controlplane.CreateTargetInput{
		Name: *name, BaseURL: *baseURL, CredentialRef: *credentialRef,
		AllowPrivateNetwork: *allowLocal,
	}
	var target controlplane.Target
	path := "/v1/projects/" + value.ProjectID + "/targets"
	if err := client.mutate(ctx, http.MethodPost, path, idempotencyKey(), input, &target); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	value.TargetID = target.ID
	if err := saveConfig(*configPath, value); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	fmt.Fprintf(stdout, "Added target %s (%s)\n", target.ID, target.BaseURL)
	return exitOK
}

func commandRun(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("run", stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration path")
	targetID := flags.String("target", "", "target ID (defaults to last added)")
	profile := flags.String("profile", "startup-r4", "evaluation profile")
	output := flags.String("output", "", "write signed report JSON to this path")
	asJSON := flags.Bool("json", false, "print signed report JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return exitUsage
	}
	value, client, code := configuredClient(*configPath, stderr)
	if code != exitOK {
		return code
	}
	if *targetID == "" {
		*targetID = value.TargetID
	}
	if *targetID == "" {
		return reportError(stderr, exitUsage, errors.New("target is required; run target add or pass --target"))
	}
	run, report, err := createRun(ctx, client, value.ProjectID, *targetID, *profile)
	if err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	value.LastRunID = run.ID
	if err := saveConfig(*configPath, value); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	if *output != "" {
		if err := os.WriteFile(*output, append(reportJSON, '\n'), 0o600); err != nil {
			return reportError(stderr, exitServiceError, fmt.Errorf("write report: %w", err))
		}
	}
	if *asJSON {
		fmt.Fprintln(stdout, string(reportJSON))
	} else {
		fmt.Fprintf(stdout, "Run %s: %s (%s)\n", run.ID, report.Decision, report.Findings[0].Summary)
	}
	return exitOK
}

func commandReportVerify(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("report verify", stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration path")
	publicKey := flags.String("key", "", "base64 Ed25519 public key")
	if flags.Parse(args) != nil || flags.NArg() != 1 {
		fmt.Fprintln(stderr, "report verify requires one report JSON file")
		return exitUsage
	}
	body, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return reportError(stderr, exitVerifyFailed, fmt.Errorf("read report: %w", err))
	}
	var report controlplane.Report
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return reportError(stderr, exitVerifyFailed, fmt.Errorf("decode report: %w", err))
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return reportError(stderr, exitVerifyFailed, errors.New("report file must contain exactly one JSON value"))
	}
	if *publicKey == "" {
		value, err := loadConfig(*configPath)
		if err != nil {
			return reportError(stderr, exitVerifyFailed, err)
		}
		*publicKey = value.PublicKey
	}
	key, err := controlplane.ParsePublicKey(*publicKey)
	if err != nil {
		return reportError(stderr, exitVerifyFailed, err)
	}
	if err := validateReport(report); err != nil {
		return reportError(stderr, exitVerifyFailed, err)
	}
	if err := controlplane.VerifyReport(report, key); err != nil {
		return reportError(stderr, exitVerifyFailed, err)
	}
	fmt.Fprintf(stdout, "Verified report %s with %s\n", report.ReportID, report.Signature.KeyID)
	return exitOK
}

func commandBaselineSet(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("baseline set", stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration path")
	runID := flags.String("run", "", "completed run ID (defaults to last run)")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return exitUsage
	}
	value, client, code := configuredClient(*configPath, stderr)
	if code != exitOK {
		return code
	}
	if *runID == "" {
		*runID = value.LastRunID
	}
	if *runID == "" {
		return reportError(stderr, exitUsage, errors.New("run ID is required"))
	}
	var baseline controlplane.Baseline
	path := "/v1/projects/" + value.ProjectID + "/baseline"
	if err := client.mutate(ctx, http.MethodPut, path, idempotencyKey(), map[string]string{"runId": *runID}, &baseline); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	fmt.Fprintf(stdout, "Baseline set to run %s\n", baseline.RunID)
	return exitOK
}

func commandCI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("ci", stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration path")
	against := flags.String("against", "baseline", "comparison source (baseline)")
	targetID := flags.String("target", "", "target ID (defaults to last added)")
	profile := flags.String("profile", "startup-r4", "evaluation profile")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *against != "baseline" {
		return exitUsage
	}
	value, client, code := configuredClient(*configPath, stderr)
	if code != exitOK {
		return code
	}
	if *targetID == "" {
		*targetID = value.TargetID
	}
	if *targetID == "" {
		return reportError(stderr, exitUsage, errors.New("target is required"))
	}
	var baseline controlplane.Baseline
	if err := client.get(ctx, "/v1/projects/"+value.ProjectID+"/baseline", &baseline); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	var baselineReport controlplane.Report
	if err := client.get(ctx, "/v1/runs/"+baseline.RunID+"/report", &baselineReport); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	run, current, err := createRun(ctx, client, value.ProjectID, *targetID, *profile)
	if err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	key, err := controlplane.ParsePublicKey(value.PublicKey)
	if err != nil {
		return reportError(stderr, exitVerifyFailed, err)
	}
	if err := controlplane.VerifyReport(baselineReport, key); err != nil {
		return reportError(stderr, exitVerifyFailed, errors.New("baseline report signature verification failed"))
	}
	if current.Signature == nil || current.Coverage.Completed < current.Coverage.Selected {
		fmt.Fprintf(stdout, "CI gate failed: run %s has incomplete coverage\n", run.ID)
		return exitGateFailed
	}
	if err := controlplane.VerifyReport(current, key); err != nil {
		return reportError(stderr, exitVerifyFailed, errors.New("current report signature verification failed"))
	}
	value.LastRunID = run.ID
	if err := saveConfig(*configPath, value); err != nil {
		return reportError(stderr, exitServiceError, err)
	}
	if regressed(baselineReport, current) || current.Decision == "not_ready" || current.Decision == "incomplete" {
		fmt.Fprintf(stdout, "CI gate failed: run %s is %s\n", run.ID, current.Decision)
		return exitGateFailed
	}
	fmt.Fprintf(stdout, "CI gate passed: run %s is %s\n", run.ID, current.Decision)
	return exitOK
}

func createRun(ctx context.Context, client *apiClient, projectID, targetID, profile string) (controlplane.Run, controlplane.Report, error) {
	var run controlplane.Run
	input := controlplane.CreateRunInput{TargetID: targetID, Profile: profile}
	if err := client.mutate(ctx, http.MethodPost, "/v1/projects/"+projectID+"/runs", idempotencyKey(), input, &run); err != nil {
		return run, controlplane.Report{}, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for run.Status == "queued" || run.Status == "running" {
		select {
		case <-waitCtx.Done():
			return run, controlplane.Report{}, errors.New("timed out waiting for run completion")
		case <-time.After(250 * time.Millisecond):
		}
		if err := client.get(waitCtx, "/v1/runs/"+run.ID, &run); err != nil {
			return run, controlplane.Report{}, err
		}
	}
	if run.Status != "completed" {
		return run, controlplane.Report{}, fmt.Errorf("run ended in %s state", run.Status)
	}
	var report controlplane.Report
	if err := client.get(ctx, "/v1/runs/"+run.ID+"/report", &report); err != nil {
		return run, report, err
	}
	return run, report, nil
}

func validateReport(report controlplane.Report) error {
	if report.SchemaVersion != controlplane.SchemaVersion {
		return fmt.Errorf("unsupported report schema version %q", report.SchemaVersion)
	}
	if report.ReportID == "" || report.RunID == "" || len(report.ManifestSHA256) != 64 {
		return errors.New("report is missing required contract fields")
	}
	if _, err := hex.DecodeString(report.ManifestSHA256); err != nil {
		return errors.New("report manifestSha256 is not lowercase hexadecimal")
	}
	if report.ManifestSHA256 != strings.ToLower(report.ManifestSHA256) {
		return errors.New("report manifestSha256 is not lowercase hexadecimal")
	}
	validDecisions := map[string]bool{"ready": true, "conditional": true, "not_ready": true, "incomplete": true}
	if !validDecisions[report.Decision] {
		return errors.New("report decision is not valid")
	}
	if report.Coverage.Completed > report.Coverage.Selected || len(report.Findings) != report.Coverage.Completed {
		return errors.New("report coverage is internally inconsistent")
	}
	return nil
}

func regressed(baseline, current controlplane.Report) bool {
	rank := map[string]int{"ready": 0, "conditional": 1, "not_ready": 2, "incomplete": 3}
	if rank[current.Decision] > rank[baseline.Decision] {
		return true
	}
	baselineOutcomes := make(map[string]string, len(baseline.Findings))
	for _, finding := range baseline.Findings {
		baselineOutcomes[finding.RuleID] = finding.Outcome
	}
	outcomeRank := map[string]int{"pass": 0, "not_applicable": 0, "warning": 1, "inconclusive": 2, "fail": 3, "platform_error": 4}
	for _, finding := range current.Findings {
		if prior, ok := baselineOutcomes[finding.RuleID]; ok && outcomeRank[finding.Outcome] > outcomeRank[prior] {
			return true
		}
	}
	return false
}

func configuredClient(configPath string, stderr io.Writer) (config, *apiClient, int) {
	value, err := loadConfig(configPath)
	if err != nil {
		return config{}, nil, reportError(stderr, exitUsage, err)
	}
	client, err := newAPIClient(value.APIURL)
	if err != nil {
		return config{}, nil, reportError(stderr, exitUsage, err)
	}
	return value, client, exitOK
}

func idempotencyKey() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(value[:])
}

func newFlagSet(name string, output io.Writer) *flag.FlagSet {
	value := flag.NewFlagSet(name, flag.ContinueOnError)
	value.SetOutput(output)
	return value
}

func reportError(output io.Writer, code int, err error) int {
	fmt.Fprintf(output, "flightcheck: %s\n", controlplane.Redact(err.Error()))
	return code
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, `Usage:
  flightcheck init [--api URL] [--name NAME] [--token TOKEN]
  flightcheck target add --name NAME --url FHIR_URL [--allow-local-demo]
  flightcheck run [--target ID] [--profile startup-r4] [--json] [--output FILE]
  flightcheck report verify [--key BASE64_PUBLIC_KEY] REPORT.json
  flightcheck baseline set [--run ID]
  flightcheck ci --against baseline [--target ID]

Exit codes: 0 success, 1 CI gate failed, 2 usage/configuration,
3 service/transport failure, 4 report verification failure.`)
}
