package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/pkg/models"
)

// Client represents the GatePlane service client
type Client struct {
	httpClient *http.Client
	baseURL    string
	jwt        string
}

// CustomTransport wraps the default transport and modifies the User-Agent header
type CustomUserAgentTransport struct {
	Transport http.RoundTripper
	UserAgent string
}

func (c *CustomUserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add the custom User-Agent header to each request
	req.Header.Set("User-Agent", c.UserAgent)

	// Call the original Transport (which will send the request)
	return c.Transport.RoundTrip(req)
}

type NotificationType string

const (
	Request NotificationType = "request"
	Approve NotificationType = "approval"
	Claim   NotificationType = "claim"
	Test    NotificationType = "test"
)

// NewClient creates a new service client
func NewClient(version string, commitHash string, buildDate string) (*Client, error) {
	cfg := config.GetConfig()

	if cfg.Service.JWT == "" {
		return nil, fmt.Errorf("service JWT not configured")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &CustomUserAgentTransport{
				UserAgent: fmt.Sprintf("GatePlane CLI/%s - <%s> %s", version, commitHash[:8], buildDate),
				Transport: http.DefaultTransport,

				// Transport: &debug.DebugTransport{
				// 	Transport: http.DefaultTransport,
				// },
			},
		},
		baseURL: config.ServiceAddress,
		jwt:     cfg.Service.JWT,
	}, nil
}

// Ping sends a GET request to the /api/ping endpoint
func (c *Client) Ping() error {
	if c == nil {
		return fmt.Errorf("service client not initialized")
	}

	url := fmt.Sprintf("%s/api/ping", c.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.jwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ping failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// SendRequestNotification sends a POST request to the /api/notification/request endpoint
func (c *Client) SendNotification(response *models.RequestServiceResponse, type_ NotificationType) error {
	if c == nil {
		return fmt.Errorf("service client not initialized")
	}

	if response == nil && type_ != Test {
		return fmt.Errorf("body is nil and notification type is not 'test'")
	}

	// Serialize the response to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal request service response: %w", err)
	}

	// fmt.Println(string(jsonData))
	url := fmt.Sprintf("%s/api/notification/%s", c.baseURL, type_)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create notification request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.jwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
