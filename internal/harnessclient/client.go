package harnessclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Client interface {
	GetCatalogServices(ctx context.Context) ([]CatalogService, error)
	GetChaosCoverage(ctx context.Context, serviceName string) (*ChaosCoverageSummary, error)
	GetSRMSummary(ctx context.Context, serviceName string) (*SRMSummary, error)
	GetSTOSummary(ctx context.Context, serviceName string) (*STOSummary, error)
	GetFlagDetails(ctx context.Context, flagKey string) (*Flag, error)
}

type HTTPClient struct {
	baseURL   string
	accountID string
	apiKey    string
	httpClient *http.Client
}

func NewHTTPClient(baseURL, accountID, apiKey string) *HTTPClient {
	return &HTTPClient{
		baseURL:   baseURL,
		accountID: accountID,
		apiKey:    apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *HTTPClient) GetCatalogServices(ctx context.Context) ([]CatalogService, error) {
	// In production, would call IDP / Backstage catalog REST API or GraphQL endpoint.
	// Fall back to error or mock adapter if not configured with active API token.
	if c.apiKey == "" || c.apiKey == "mock-key" {
		return nil, fmt.Errorf("HTTP client requires valid Harness API Key; use MockClient for local testing")
	}
	return nil, fmt.Errorf("harness IDP catalog HTTP call unimplemented in v0.1 core")
}

func (c *HTTPClient) GetChaosCoverage(ctx context.Context, serviceName string) (*ChaosCoverageSummary, error) {
	if c.apiKey == "" || c.apiKey == "mock-key" {
		return nil, fmt.Errorf("HTTP client requires valid Harness API Key; use MockClient for local testing")
	}
	return nil, fmt.Errorf("harness Chaos HTTP call unimplemented in v0.1 core")
}

func (c *HTTPClient) GetSRMSummary(ctx context.Context, serviceName string) (*SRMSummary, error) {
	if c.apiKey == "" || c.apiKey == "mock-key" {
		return nil, fmt.Errorf("HTTP client requires valid Harness API Key; use MockClient for local testing")
	}
	return nil, fmt.Errorf("harness SRM HTTP call unimplemented in v0.1 core")
}

func (c *HTTPClient) GetSTOSummary(ctx context.Context, serviceName string) (*STOSummary, error) {
	if c.apiKey == "" || c.apiKey == "mock-key" {
		return nil, fmt.Errorf("HTTP client requires valid Harness API Key; use MockClient for local testing")
	}
	return nil, fmt.Errorf("harness STO HTTP call unimplemented in v0.1 core")
}

func (c *HTTPClient) GetFlagDetails(ctx context.Context, flagKey string) (*Flag, error) {
	if c.apiKey == "" || c.apiKey == "mock-key" {
		return nil, fmt.Errorf("HTTP client requires valid Harness API Key; use MockClient for local testing")
	}
	return nil, fmt.Errorf("harness FME HTTP call unimplemented in v0.1 core")
}
