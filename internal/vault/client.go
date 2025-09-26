package vault

import (
	"encoding/json"
	"fmt"
	"os"
	// "strconv"
	"strings"

	// "github.com/gateplane-io/client-cli/pkg/errors"
	"github.com/gateplane-io/client-cli/pkg/models"
	vault "github.com/hashicorp/vault/api"

	"github.com/gateplane-io/vault-plugins/pkg/models/base"
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

	if config.Token != "" {
		client.SetToken(config.Token)
	} else if token := os.Getenv("VAULT_TOKEN"); token != "" {
		client.SetToken(token)
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

func isGatePlanePlugin(pluginType string) bool {
	return strings.Contains(pluginType, "gateplane") ||
		strings.Contains(pluginType, "policy-gate") ||
		strings.Contains(pluginType, "okta-group-gate")
}
