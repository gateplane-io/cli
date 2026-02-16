package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/internal/service"
	"github.com/gateplane-io/client-cli/internal/vault"
	project_models "github.com/gateplane-io/client-cli/pkg/models"

	"github.com/fatih/color"
	"github.com/gateplane-io/vault-plugins/pkg/models"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// Output formats
const (
	OutputFormatJSON  = "json"
	OutputFormatYAML  = "yaml"
	OutputFormatTable = "table"
	OutputFormatEnv   = "env"
)

// getEffectiveOutputFormat returns the output format to use, checking flag -> config -> default
func getEffectiveOutputFormat() string {
	if outputFormat != "" {
		return outputFormat
	}
	cfg := config.GetConfig()
	if cfg.Defaults.OutputFormat != "" {
		return cfg.Defaults.OutputFormat
	}
	return OutputFormatTable
}

// createVaultClient creates a vault client using the global configuration
func createVaultClient() (*vault.Client, error) {
	return vault.NewClient(getVaultClientConfig())
}

// formatOutput handles the common output formatting logic used across commands
func formatOutput(data interface{}, format string) error {
	switch format {
	case OutputFormatJSON:
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		fmt.Println(string(jsonData))

	case OutputFormatYAML:
		yamlData, err := yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %w", err)
		}
		fmt.Print(string(yamlData))

	default:
		return fmt.Errorf("unsupported output format for generic data: %s", format)
	}
	return nil
}

// resolveGateFromArgs resolves gate from command arguments with fallback to config
func resolveGateFromArgs(args []string) (string, error) {
	cfg := config.GetConfig()

	var gate string
	if len(args) > 0 {
		gate = config.ResolveGatePath(args[0])
	} else {
		gate = cfg.Defaults.Gate
		if gate == "" {
			return "", fmt.Errorf("no gate specified. Provide gate as argument or set default with 'gateplane config set default-gate'")
		}
		gate = config.ResolveGatePath(gate)
	}
	return gate, nil
}

// isInteractiveMode determines if we should use interactive mode based on flags and TTY
func isInteractiveMode(interactive bool, hasArgs bool, hasRequiredFlags bool) bool {
	// Use interactive mode if:
	// 1. Explicitly requested with -i flag
	// 2. No arguments/required flags provided AND we have a TTY
	return interactive || (!hasArgs && !hasRequiredFlags && term.IsTerminal(int(os.Stdin.Fd())))
}

// printSuccessMessage prints a success message with green checkmark
func printSuccessMessage(message string, args ...interface{}) {
	color.Green("✓ "+message, args...)
}

// printSuccessMessage prints a success message with green checkmark
func printFailedMessage(message string, args ...interface{}) {
	color.Red("× "+message, args...)
}

// // formatRequestStatus returns a colored string representation of request status
// // This function is now deprecated in favor of req.FormatStatus() method
// func formatRequestStatus(req *models.Request) string {
// 	return req.FormatStatus()
// }

// formatRequestStatus returns a colored string representation of request status
func formatRequestStatus(status models.AccessRequestStatus) string {
	switch status {
	case models.Pending:
		return color.YellowString("Pending")
	case models.Approved:
		return color.GreenString("Approved")
	case models.Active:
		return color.BlueString("Active")
	case models.Expired:
		return color.RedString("Expired")
	case models.Abandoned:
		return color.RedString("Abandoned")
	case models.Rejected:
		return color.RedString("Rejected")
	case models.Revoked:
		return color.RedString("Revoked")
	default:
		return color.WhiteString(status.String())
	}
}

// formatGateDisplay formats gate path with optional default gate highlighting
func formatGateDisplay(gatePath string) string {
	cfg := config.GetConfig()
	if gatePath == cfg.Defaults.Gate {
		return color.CyanString("%s *", gatePath)
	}
	return gatePath
}

// wrapError wraps an error with context information
func wrapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to %s: %w", operation, err)
}

func getVaultClientConfig() *vault.Config {
	cfg := config.GetConfig()
	vaultConfig := &vault.Config{
		Address:   cfg.Vault.Address,
		Token:     cfg.Vault.Token,
		Namespace: cfg.Vault.Namespace,
	}

	// Command-line flags override config and env vars
	if vaultAddr != "" {
		vaultConfig.Address = vaultAddr
	}
	if vaultToken != "" {
		vaultConfig.Token = vaultToken
	}

	return vaultConfig
}

// sendNotificationWithRetry sends a notification with consistent error handling
// Logs warnings instead of failing if service is unavailable or notification fails
func sendNotificationWithRetry(svcClient *service.Client, vaultClient *vault.Client, req *project_models.Request, gate string, notificationType service.NotificationType) error {
	if svcClient == nil {
		return nil
	}

	accessStruct, err := vaultClient.GetPolicyGateAccessStruct(gate)
	if err != nil {
		fmt.Printf("Warning: failed to get gate access struct for notification: %v\n", err)
		return nil
	}

	if err := svcClient.SendNotification(&project_models.RequestServiceResponse{
		Request: req.AccessRequestResponse,
		Gate:    *req.Gate,
		Access:  *accessStruct,
	}, notificationType); err != nil {
		fmt.Printf("Warning: failed to send notification: %v\n", err)
	}

	return nil
}
