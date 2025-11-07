package vault

import (
	"encoding/json"
	"fmt"
	// "time"
	"os"
	// "strconv"
	"strings"
	"github.com/mitchellh/go-homedir"

	"github.com/gateplane-io/client-cli/pkg/errors"
	"github.com/gateplane-io/client-cli/pkg/models"
	vault "github.com/hashicorp/vault/api"

	"github.com/gateplane-io/vault-plugins/pkg/responses"
	base "github.com/gateplane-io/vault-plugins/pkg/models"
)

// Helper function to convert string to RequestStatus
func stringToRequestStatus(status string) base.AccessRequestStatus {
	var rs base.AccessRequestStatus
	data, _ := json.Marshal(status)
	if err := rs.UnmarshalJSON(data); err != nil {
		// Return default status on unmarshal error
		return base.Pending
	}
	return rs
}

// Client wraps the Vault client with GatePlane-specific functionality
type Client struct {
	client *vault.Client
	config *Config
}

// Config holds the configuration for connecting to Vault
type Config struct {
	Address   string
	Token     string
	Namespace string
}

// NewClient creates a new Vault client with the provided configuration
func NewClient(config *Config) (*Client, error) {
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = config.Address

	if config.Address == "" {
		if addr := os.Getenv("VAULT_ADDR"); addr != "" {
			vaultConfig.Address = addr
		} else {
			return nil, fmt.Errorf("vault address not configured")
		}
	}

	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	// Read the vault token from conf / env / vault login file
	home, err := homedir.Dir()
	vaultTokenFile := fmt.Sprintf("%s/.vault-token", home)
	data, err := os.ReadFile(vaultTokenFile)
	if config.Token != "" {
		client.SetToken(config.Token)
	} else if token := os.Getenv("VAULT_TOKEN"); token != "" {
		client.SetToken(token)
	} else if err == nil && string(data) != "" {
		client.SetToken(string(data))
	}

	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	} else if namespace := os.Getenv("VAULT_NAMESPACE"); namespace != "" {
		client.SetNamespace(namespace)
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

func (c *Client) VaultClient() *vault.Client {
	return c.client
}

func (c *Client) DiscoverGates() ([]*models.Gate, error) {
	auths, err := c.client.Sys().ListAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to list auth methods: %w", err)
	}

	var gates []*models.Gate
	for path, auth := range auths {
		if isGatePlanePlugin(auth.Type) {
			gateType := models.PolicyGate
			if strings.Contains(auth.Type, "okta") {
				gateType = models.OktaGroupGate
			}

			gate := &models.Gate{
				Path:        "auth/" + strings.TrimSuffix(path, "/"),
				Type:        gateType,
				Description: auth.Description,
			}
			gates = append(gates, gate)
		}
	}

	return gates, nil
}

func (c *Client) CreateRequest(gate string, justification string) error {
	path := fmt.Sprintf("%s/request", gate)
	data := map[string]interface{}{
		"justification": justification,
		// "ttl": ttl,
	}

	_, err := c.client.Logical().Write(path, data)
	if err != nil {
		return errors.WrapVaultError("create request", gate, err)
	}

	return nil
}

func (c *Client) GetRequestStatus(gate string) (*responses.AccessRequestResponse, error) {
	path := fmt.Sprintf("%s/request", gate)

	resp, err := c.client.Logical().Read(path)
	if err != nil {
		return nil, errors.WrapVaultError("get request status", gate, err)
	}

	if resp == nil || resp.Data == nil {
		return nil, nil // Return nil request instead of error for no active request
	}

	respJson, err := json.Marshal(resp.Data)
	if err != nil {
	    fmt.Println("Error marshaling data:", err)
        return nil, err
	}

	var accessRequest responses.AccessRequestResponse
    err = json.Unmarshal([]byte(respJson), &accessRequest)
    if err != nil {
        fmt.Println("Error unmarshaling JSON:", err)
        return nil, err
    }

	// Parse the response using the new structured approach
	// vaultResp, err := models.ParseRequestResponse(resp.Data)
	// if err != nil {
		// return nil, errors.WrapVaultError("parse request response", gate, err)
	// }

	// Get gate configuration to find required_approvals
	// configPath := fmt.Sprintf("%s/config", gate)
	// configResp, err := c.client.Logical().Read(configPath)
	// var requiredApprovals int
	// if err == nil && configResp != nil && configResp.Data != nil {
	// 	if gateConfig, err := models.ParseGateConfig(configResp.Data); err == nil {
	// 		requiredApprovals = gateConfig.RequiredApprovals
	// 	}
	// }

	return &accessRequest, nil
}


func isGatePlanePlugin(pluginType string) bool {
	return strings.Contains(pluginType, "gateplane") &&
		strings.Contains(pluginType, "policy-gate") ||
		strings.Contains(pluginType, "okta-group-gate")
}
