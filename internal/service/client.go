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

type NotificationType string

const (
	Request NotificationType = "request"
	Approve NotificationType = "approval"
	Claim   NotificationType = "claim"
)

// NewClient creates a new service client
func NewClient() (*Client, error) {
	cfg := config.GetConfig()

	if cfg.Service.JWT == "" {
		return nil, fmt.Errorf("service JWT not configured")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
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

	fmt.Printf("✓ Service ping successful\n")
	return nil
}

// Ping sends a GET request to the /api/ping endpoint
func (c *Client) TestNotification() error {
	if c == nil {
		return fmt.Errorf("service client not initialized")
	}

	bodyJSON, err := json.Marshal(map[string]string{})
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/api/notification/test", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.jwt)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GatePlane CLI, v0.0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ping failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("✓ Service notification successful\n")
	return nil
}

// SendRequestNotification sends a POST request to the /api/notification/request endpoint
func (c *Client) SendRequestNotification(response *models.RequestServiceResponse, type_ NotificationType) error {
	if c == nil {
		return fmt.Errorf("service client not initialized")
	}

	if response == nil {
		return fmt.Errorf("request service response is nil")
	}

	// Serialize the response to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal request service response: %w", err)
	}

	url := fmt.Sprintf("%s/api/notification/%s", c.baseURL, type_)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create notification request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.jwt)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GatePlane CLI, v0.0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("✓ Request notification sent for gate %s (request ID: %s)\n", response.Path, response.ID)
	return nil
}
