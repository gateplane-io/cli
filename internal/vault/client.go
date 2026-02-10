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

	base "github.com/gateplane-io/vault-plugins/pkg/models"
	"github.com/gateplane-io/vault-plugins/pkg/responses"
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
	auths, err := c.client.Sys().ListMounts()
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
				Path:        strings.TrimSuffix(path, "/"),
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
		// Return nil request instead of error for no active request
		return nil, nil
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

func (c *Client) ListAllRequestsForGate(path string) ([]*models.Request, error) {
	listPath := fmt.Sprintf("%s/request", path)

	resp, err := c.client.Logical().List(listPath)
	if err != nil {
		return nil, errors.WrapVaultError("list requests", path, err)
	}

	if resp == nil || resp.Data == nil {
		return []*models.Request{}, nil // Return empty array if no requests
	}

	// The response should contain a map with request IDs as keys
	requestsMap, ok := resp.Data["key_info"].(map[string]interface{})
	if !ok {
		return []*models.Request{}, nil
	}

	var requests []*models.Request

	// Create gate info for all requests
	gate := &models.Gate{
		Path: strings.TrimSuffix(path, "/"),
	}

	// Determine gate type by checking the plugin type
	mounts, err := c.client.Sys().ListMounts()
	if err == nil {
		if auth, exists := mounts[path+"/"]; exists {
			if strings.Contains(auth.Type, "okta") {
				gate.Type = models.OktaGroupGate
			} else {
				gate.Type = models.PolicyGate
			}
			gate.Description = auth.Description
		}
	}

	for _, requestData := range requestsMap {
		map_, ok := requestData.(map[string]interface{})
		if !ok {
			continue // Skip if not a valid request map
		}

		// Convert the request data to JSON
		requestJson, err := json.Marshal(map_)
		if err != nil {
			continue // Skip if we can't marshal
		}

		// Parse into AccessRequestResponse
		var accessRequest responses.AccessRequestResponse
		err = json.Unmarshal(requestJson, &accessRequest)
		if err != nil {
			continue // Skip if we can't unmarshal
		}

		// Create the combined Request model
		request := &models.Request{
			AccessRequestResponse: &accessRequest,
			Gate:                  gate,
		}

		requests = append(requests, request)
	}

	return requests, nil
}

func (c *Client) ApproveRequest(gate string, requestorID string) error {
	path := fmt.Sprintf("%s/approve", gate)
	data := map[string]interface{}{
		"requestor_id": requestorID,
	}

	_, err := c.client.Logical().Write(path, data)
	if err != nil {
		return errors.WrapVaultError("approve request", gate, err)
	}

	return nil
}

func (c *Client) GetSelf() (*models.Self, error) {
	// Get token information using LookupSelf - this contains both entity and alias info
	secret, err := c.client.Auth().Token().LookupSelf()
	if err != nil {
		return nil, errors.WrapVaultError("lookup self token", "", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, errors.NewVaultError("lookup self token", "", fmt.Errorf("no token data found"))
	}

	self := &models.Self{}

	// Initialize embedded structs
	self.Entity = &models.Entity{}
	self.EntityAlias = &models.EntityAlias{}

	// Extract entity ID
	if entityID, ok := secret.Data["entity_id"].(string); ok {
		self.Entity.ID = entityID
	}

	// Extract alias ID
	if aliasID, ok := secret.Data["alias_id"].(string); ok {
		self.EntityAlias.ID = aliasID
	}

	// Extract entity name (from display_name or metadata)
	if displayName, ok := secret.Data["display_name"].(string); ok {
		self.Entity.Name = displayName
		self.DisplayName = displayName
	}

	// Extract policies
	if policies, ok := secret.Data["policies"].([]interface{}); ok {
		for _, policy := range policies {
			if policyStr, ok := policy.(string); ok {
				self.Entity.Policies = append(self.Entity.Policies, policyStr)
			}
		}
	}

	// Extract metadata
	if meta, ok := secret.Data["meta"].(map[string]interface{}); ok {
		self.Entity.Metadata = meta

		// Try to get username from metadata
		if username, exists := meta["username"].(string); exists {
			self.Username = username
		}
	}

	// Extract alias information from the token response
	if aliasMeta, ok := secret.Data["alias"].(map[string]interface{}); ok {
		if name, ok := aliasMeta["name"].(string); ok {
			self.EntityAlias.Name = name
		}
		if mount, ok := aliasMeta["mount"].(string); ok {
			self.EntityAlias.MountPath = mount
		}
		if canonicalMount, ok := aliasMeta["canonical_mount"].(string); ok {
			self.EntityAlias.CanonicalMount = canonicalMount
		}
		if mountAccessor, ok := aliasMeta["mount_accessor"].(string); ok {
			self.EntityAlias.MountAccessor = mountAccessor
		}
		if mountType, ok := aliasMeta["mount_type"].(string); ok {
			self.EntityAlias.MountType = mountType
		}
	}

	return self, nil
}

func (c *Client) ClaimAccess(gate string) (map[string]interface{}, error) {
	path := fmt.Sprintf("%s/claim", gate)

	resp, err := c.client.Logical().Write(path, nil)
	if err != nil {
		return nil, errors.WrapVaultError("claim access", gate, err)
	}

	// Return the data from the response
	if resp != nil && resp.Data != nil {
		return resp.Data, nil
	}

	// If no data, return nil
	return nil, nil
}

func isGatePlanePlugin(pluginType string) bool {
	return strings.Contains(pluginType, "gateplane") &&
		strings.Contains(pluginType, "policy-gate") ||
		strings.Contains(pluginType, "okta-group-gate")
}
