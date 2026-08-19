package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	controlplane "github.com/Arshiamk/fhir-flightcheck/services/control-plane"
)

func main() {
	if len(os.Args) > 1 {
		if len(os.Args) == 2 && os.Args[1] == "generate-signing-key" {
			if err := generateSigningKey(os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "control-plane: %s\n", controlplane.Redact(err.Error()))
				os.Exit(1)
			}
			return
		}
		fmt.Fprintln(os.Stderr, "usage: control-plane [generate-signing-key]")
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("control plane failed", "error", controlplane.Redact(err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	address := envOr("FLIGHTCHECK_LISTEN_ADDR", "127.0.0.1:8080")
	databaseURL := os.Getenv("FLIGHTCHECK_DATABASE_URL")
	storageMode := os.Getenv("FLIGHTCHECK_STORAGE_MODE")
	durable := databaseURL != ""
	if storageMode == "" {
		if durable {
			storageMode = "postgres"
		} else {
			return errors.New("set FLIGHTCHECK_DATABASE_URL or explicitly set FLIGHTCHECK_STORAGE_MODE=memory")
		}
	}
	if storageMode != "postgres" && storageMode != "memory" {
		return fmt.Errorf("unsupported FLIGHTCHECK_STORAGE_MODE %q", storageMode)
	}
	if storageMode == "postgres" && databaseURL == "" {
		return errors.New("FLIGHTCHECK_DATABASE_URL is required for postgres storage")
	}
	if storageMode == "memory" && databaseURL != "" {
		return errors.New("memory storage cannot be combined with FLIGHTCHECK_DATABASE_URL")
	}
	loopback := isLoopbackAddress(address)
	requireAPIAuth := durable || !loopback
	apiToken, workerToken := os.Getenv("FLIGHTCHECK_API_TOKEN"), os.Getenv("FLIGHTCHECK_WORKER_TOKEN")
	if requireAPIAuth && len(apiToken) < 32 {
		return errors.New("FLIGHTCHECK_API_TOKEN must contain at least 32 characters in durable or non-loopback mode")
	}
	if len(workerToken) < 32 {
		return errors.New("FLIGHTCHECK_WORKER_TOKEN must contain at least 32 characters")
	}
	if apiToken != "" && apiToken == workerToken {
		return errors.New("FLIGHTCHECK_API_TOKEN and FLIGHTCHECK_WORKER_TOKEN must be distinct")
	}
	signer, err := loadSigner(durable)
	if err != nil {
		return fmt.Errorf("invalid signing configuration: %w", err)
	}
	if os.Getenv("FLIGHTCHECK_SIGNING_KEY") == "" {
		logEphemeralKeyWarning(logger, signer)
	}
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startupCancel()
	var repository controlplane.Repository
	var closeRepository func()
	if storageMode == "postgres" {
		postgres, err := controlplane.NewPostgresRepository(startupCtx, databaseURL, "org_local")
		if err != nil {
			return err
		}
		repository, closeRepository = postgres, postgres.Close
	} else {
		if !loopback {
			return errors.New("memory mode is restricted to an explicit loopback listen address")
		}
		repository, closeRepository = controlplane.NewMemoryRepository(), func() {}
	}
	defer closeRepository()
	catalog, err := controlplane.LoadRuleCatalog(envOr("FLIGHTCHECK_RULE_PACK_DIR", "packages/rule-packs"))
	if err != nil {
		return err
	}
	natsURL := os.Getenv("FLIGHTCHECK_NATS_URL")
	if natsURL == "" {
		return errors.New("FLIGHTCHECK_NATS_URL is required for asynchronous evaluation")
	}
	subject := envOr("FLIGHTCHECK_JOB_SUBJECT", controlplane.DefaultJobSubject)
	publisher, err := controlplane.NewNATSPublisher(startupCtx, natsURL, controlplane.DefaultJobStream, subject)
	if err != nil {
		return err
	}
	defer publisher.Close()
	allowLocal := strings.EqualFold(os.Getenv("FLIGHTCHECK_ALLOW_LOCAL_DEMO"), "true")
	fixtureDir := resolveFixtureDir()
	service := &controlplane.Service{
		Repository:     repository,
		Signer:         signer,
		Catalog:        catalog,
		JobSubject:     subject,
		OrganizationID: "org_local",
		FixtureDir:     fixtureDir,
		Checker: controlplane.CapabilityChecker{
			Policy:  controlplane.URLPolicy{AllowLocalDemo: allowLocal, ResolveTimeout: 3 * time.Second},
			Timeout: 10 * time.Second,
		},
	}
	dispatchCtx, stopDispatch := context.WithCancel(context.Background())
	defer stopDispatch()
	go (&controlplane.OutboxDispatcher{Repository: repository, Publisher: publisher, Logger: logger}).Run(dispatchCtx)
	server := &http.Server{
		Addr: address,
		Handler: controlplane.NewHandlerWithAuth(service, logger, controlplane.AuthConfig{
			RequireAPIAuth: requireAPIAuth, APIToken: apiToken, WorkerToken: workerToken,
		}),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second,
		WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second,
		BaseContext: func(net.Listener) context.Context { return context.Background() },
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("control plane listening", "address", address, "allow_local_demo", allowLocal)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	select {
	case <-stop.Done():
	case err := <-serverErrors:
		return fmt.Errorf("serve HTTP: %w", err)
	}
	stopDispatch()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}

type generatedSigningKey struct {
	Algorithm  string `json:"algorithm"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	KeyID      string `json:"keyId"`
}

func generateSigningKey(output io.Writer) error {
	signer, err := controlplane.NewEphemeralSigner()
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(generatedSigningKey{
		Algorithm: "Ed25519", PrivateKey: signer.PrivateKeyBase64(),
		PublicKey: signer.PublicKeyBase64(), KeyID: signer.KeyID(),
	})
}

func logEphemeralKeyWarning(logger *slog.Logger, signer *controlplane.Signer) {
	logger.Warn("using ephemeral signing key; reports cannot be verified after restart", "key_id", signer.KeyID())
}

func loadSigner(durable bool) (*controlplane.Signer, error) {
	if encoded := os.Getenv("FLIGHTCHECK_SIGNING_KEY"); encoded != "" {
		return controlplane.ParsePrivateKey(encoded)
	}
	if durable {
		return nil, errors.New("FLIGHTCHECK_SIGNING_KEY is required in durable mode")
	}
	return controlplane.NewEphemeralSigner()
}

func resolveFixtureDir() string {
	dir := os.Getenv("FLIGHTCHECK_FIXTURE_DIR")
	if dir == "" {
		dir = "fixtures/synthea"
	}
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
