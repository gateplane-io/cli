package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gateplane-io/client-cli/pkg/models"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Config represents the main configuration structure for the GatePlane CLI
type Config struct {
	Vault    VaultConfig              `yaml:"vault"`
	Service  ServiceConfig            `yaml:"service"`
	Defaults DefaultsConfig           `yaml:"defaults"`
	Gates    []models.Gate            `yaml:"gates"`
	Profiles map[string]ProfileConfig `yaml:"profiles"`
}

// VaultConfig contains Vault server connection settings
type VaultConfig struct {
	Address   string `yaml:"address"`
	Token     string `yaml:"token"`
	Namespace string `yaml:"namespace"`
}

// ServiceConfig contains GatePlane service authentication settings
type ServiceConfig struct {
	ClientID string `mapstructure:"client_id" yaml:"client_id"`
	JWT      string `yaml:"jwt"`
	JWKS     string `yaml:"jwks"`
}

var ServiceAddress = "https://backend.gateplane.io"

// DefaultsConfig contains default values for CLI operations
type DefaultsConfig struct {
	Gate         string `yaml:"gate"`
	OutputFormat string `mapstructure:"output_format" yaml:"output_format"`
}

// ProfileConfig contains settings for a specific configuration profile
type ProfileConfig struct {
	VaultAddress string `yaml:"vault_address"`
	DefaultGate  string `yaml:"default_gate"`
	Namespace    string `yaml:"namespace,omitempty"`
}

var (
	cfg        *Config
	configFile string
	credsFile  string
	vaultFile  string
)

// Init initializes the configuration system by creating config directory and loading config file
func Init() error {
	home, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".gateplane")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile = filepath.Join(configDir, "config.yaml")
	credsFile = filepath.Join(configDir, ".credentials.yaml")
	// The file created by the 'vault login' command
	vaultFile = filepath.Join(home, ".vault-token")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Set defaults
	viper.SetDefault("defaults.output_format", "table")

	// Bind environment variables
	viper.SetEnvPrefix("GATEPLANE")
	viper.AutomaticEnv()

	// Override with environment variables
	if err := viper.BindEnv("vault.address", "VAULT_ADDR"); err != nil {
		return fmt.Errorf("failed to bind vault.address env: %w", err)
	}
	if err := viper.BindEnv("vault.token", "VAULT_TOKEN"); err != nil {
		return fmt.Errorf("failed to bind vault.token env: %w", err)
	}
	if err := viper.BindEnv("vault.namespace", "VAULT_NAMESPACE"); err != nil {
		return fmt.Errorf("failed to bind vault.namespace env: %w", err)
	}

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; create default config
		cfg = &Config{
			Defaults: DefaultsConfig{
				OutputFormat: "table",
			},
		}
		return SaveConfig()
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// If the ~/.vault-token contains a token
	// it takes priority over the hardcoded one
	_, exists := os.LookupEnv("VAULT_TOKEN")
	if !exists {
		vaultFileToken, err := ReadVaultFile()
		if err == nil {
			cfg.Vault.Token = vaultFileToken
		}
	}

	return nil
}

// GetConfig returns the current configuration, initializing it if necessary
func GetConfig() *Config {
	if cfg == nil {
		if err := Init(); err != nil {
			// Log the error but continue with default config
			fmt.Printf("Warning: failed to initialize config: %v\n", err)
		}
	}
	return cfg
}

// SaveConfig saves the current configuration to disk
func SaveConfig() error {
	viper.Set("vault", cfg.Vault)
	viper.Set("service", cfg.Service)
	viper.Set("defaults", cfg.Defaults)
	viper.Set("gates", cfg.Gates)
	viper.Set("profiles", cfg.Profiles)

	return viper.WriteConfigAs(configFile)
}

// SetVaultAddress updates the Vault address in configuration and saves it
func SetVaultAddress(address string) error {
	cfg.Vault.Address = address
	return SaveConfig()
}

// SetVaultToken updates the Vault token in configuration and saves it
func SetVaultToken(token string) error {
	cfg.Vault.Token = token
	return SaveConfig()
}

// SetDefaultGate updates the default gate in configuration and saves it
func SetDefaultGate(gate string) error {
	cfg.Defaults.Gate = gate
	return SaveConfig()
}

// AddGateAlias adds or updates a gate alias in configuration and saves it
func AddGateAlias(path, alias string, gateType models.GateType) error {
	for i, gate := range cfg.Gates {
		if gate.Path == path {
			cfg.Gates[i].Alias = alias
			return SaveConfig()
		}
	}

	cfg.Gates = append(cfg.Gates, models.Gate{
		Path:  path,
		Alias: alias,
		Type:  gateType,
	})

	return SaveConfig()
}

// GetGateByAlias retrieves a gate configuration by its alias
func GetGateByAlias(alias string) (*models.Gate, error) {
	for _, gate := range cfg.Gates {
		if gate.Alias == alias {
			return &gate, nil
		}
	}
	return nil, fmt.Errorf("gate with alias %s not found", alias)
}

// ResolveGatePath resolves a gate reference to its full path, handling aliases and direct paths
func ResolveGatePath(gateRef string) string {
	// Check if it's an alias (starts with @)
	if len(gateRef) > 0 && gateRef[0] == '@' {
		alias := gateRef[1:]
		if gate, err := GetGateByAlias(alias); err == nil {
			return gate.Path
		}
	}

	// Check if it's a known gate path
	for _, gate := range cfg.Gates {
		if gate.Path == gateRef || gate.Alias == gateRef {
			return gate.Path
		}
	}

	// Return as-is (might be a full path)
	return gateRef
}

// UseProfile switches to the specified configuration profile and saves the changes
func UseProfile(profileName string) error {
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %s not found", profileName)
	}

	if profile.VaultAddress != "" {
		cfg.Vault.Address = profile.VaultAddress
	}
	if profile.DefaultGate != "" {
		cfg.Defaults.Gate = profile.DefaultGate
	}
	if profile.Namespace != "" {
		cfg.Vault.Namespace = profile.Namespace
	}

	return SaveConfig()
}

// SetServiceJWT updates the service JWT token in configuration and saves it
func SetServiceJWT(jwt string) error {
	cfg.Service.JWT = jwt
	return SaveConfig()
}

// SetServiceJWKS updates the service JWKS in configuration and saves it
func SetServiceJWKS(jwks string) error {
	cfg.Service.JWKS = jwks
	return SaveConfig()
}

// SetServiceClientID updates the service client ID in configuration and saves it
func SetServiceClientID(clientID string) error {
	cfg.Service.ClientID = clientID
	return SaveConfig()
}

// ClearServiceAuth clears service authentication credentials and saves the configuration
func ClearServiceAuth() error {
	cfg.Service.JWT = ""
	cfg.Service.JWKS = ""
	return SaveConfig()
}

// ReadVaultFile reads the contents of the vault token file
func ReadVaultFile() (string, error) {
	if vaultFile == "" {
		// Initialize vaultFile path if not already done
		home, err := homedir.Dir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		vaultFile = filepath.Join(home, ".vault-token")
	}

	data, err := os.ReadFile(vaultFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("vault token file not found at %s", vaultFile)
		}
		return "", fmt.Errorf("failed to read vault token file: %w", err)
	} else if len(data) == 0 {
		return "", fmt.Errorf("vault token file is empty: %w", err)
	}

	return string(data), nil
}
