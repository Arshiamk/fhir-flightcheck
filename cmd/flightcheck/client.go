package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	controlplane "github.com/Arshiamk/fhir-flightcheck/services/control-plane"
)

type apiClient struct {
	baseURL string
	client  *http.Client
	token   string
}

type apiError struct {
	Status  int
	Problem controlplane.Problem
}

func (e *apiError) Error() string {
	if e.Problem.Detail != "" {
		return e.Problem.Detail
	}
	return fmt.Sprintf("control plane returned HTTP %d", e.Status)
}

func newAPIClient(baseURL string) (*apiClient, error) {
	normalized, err := controlplane.NormalizeAPIURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &apiClient{
		baseURL: normalized, client: &http.Client{Timeout: 20 * time.Second},
		token: os.Getenv("FLIGHTCHECK_API_TOKEN"),
	}, nil
}

func (c *apiClient) get(ctx context.Context, path string, output any) error {
	return c.do(ctx, http.MethodGet, path, "", nil, output)
}

func (c *apiClient) mutate(ctx context.Context, method, path, key string, input, output any) error {
	if key == "" {
		return errors.New("idempotency key is required")
	}
	return c.do(ctx, method, path, key, input, output)
}

func (c *apiClient) do(ctx context.Context, method, path, key string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("contact control plane: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read control plane response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var problem controlplane.Problem
		_ = json.Unmarshal(responseBody, &problem)
		return &apiError{Status: response.StatusCode, Problem: problem}
	}
	if output != nil && len(strings.TrimSpace(string(responseBody))) > 0 {
		if err := json.Unmarshal(responseBody, output); err != nil {
			return fmt.Errorf("decode control plane response: %w", err)
		}
	}
	return nil
}
