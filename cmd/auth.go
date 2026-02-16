package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/internal/vault"
	vault_api "github.com/hashicorp/vault/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication operations",
		Long:  "Manage authentication with Vault/OpenBao",
	}

	cmd.AddCommand(
		authLoginCmd(),
		authStatusCmd(),
		authLogoutCmd(),
		serviceCmd(),
	)

	return cmd
}

func authLoginCmd() *cobra.Command {
	var (
		vaultAddr string
		namespace string
		token     string
	)

	cmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"signin"},
		Short:   "Authenticate with Vault",
		Long:    "Authenticate with Vault/OpenBao using a token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			// Get vault address
			if vaultAddr == "" {
				vaultAddr = cfg.Vault.Address
				if vaultAddr == "" {
					if addr := os.Getenv("VAULT_ADDR"); addr != "" {
						vaultAddr = addr
					} else {
						fmt.Print("Enter Vault address: ")
						if _, err := fmt.Scanln(&vaultAddr); err != nil {
							return fmt.Errorf("failed to read vault address: %w", err)
						}
					}
				}
			}

			// Token-based authentication
			if token == "" {
				fmt.Print("Enter Vault token: ")
				tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return fmt.Errorf("failed to read token: %w", err)
				}
				fmt.Println()
				token = string(tokenBytes)
			}

			// Update config
			cfg.Vault.Address = vaultAddr
			cfg.Vault.Token = token
			if namespace != "" {
				cfg.Vault.Namespace = namespace
			}

			// Test connection
			vaultConfig := &vault.Config{
				Address:   vaultAddr,
				Token:     token,
				Namespace: namespace,
			}

			client, err := vault.NewClient(vaultConfig)
			if err != nil {
				return fmt.Errorf("failed to create vault client: %w", err)
			}

			// Try to get token info to verify auth
			tokenInfo, err := client.VaultClient().Auth().Token().LookupSelf()
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			// Save config
			if err := config.SaveConfig(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			printAuthSuccessMessage(tokenInfo)

			return nil
		},
	}

	cmd.Flags().StringVar(&vaultAddr, "address", "", "Vault address")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Vault namespace")
	cmd.Flags().StringVar(&token, "token", "", "Vault token (use with caution)")

	return cmd
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"whoami"},
		Short:   "Check authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			fmt.Printf("Vault Address: %s\n", cfg.Vault.Address)
			if cfg.Vault.Namespace != "" {
				fmt.Printf("Namespace: %s\n", cfg.Vault.Namespace)
			}

			tokenInfo, err := client.VaultClient().Auth().Token().LookupSelf()
			if err != nil {
				printFailedMessage("Not authenticated")
				return nil
			}

			printTokenInfo(tokenInfo)
			printSuccessMessage("Authenticated")
			return nil
		},
	}
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Aliases: []string{"signout"},
		Short:   "Clear stored authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()
			cfg.Vault.Token = ""

			if err := config.SaveConfig(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println("Logged out successfully")
			return nil
		},
	}
}

// printAuthSuccessMessage prints authentication success with optional username
func printAuthSuccessMessage(tokenInfo *vault_api.Secret) {
	if tokenInfo == nil || tokenInfo.Data == nil {
		fmt.Println("Successfully authenticated")
		return
	}

	username, ok := tokenInfo.Data["display_name"].(string)
	if !ok || username == "" {
		fmt.Println("Successfully authenticated")
		return
	}

	fmt.Printf("Successfully authenticated as %s\n", username)
}

// printTokenInfo prints token information in a formatted way
func printTokenInfo(tokenInfo *vault_api.Secret) {
	if tokenInfo == nil || tokenInfo.Data == nil {
		return
	}

	if username, ok := tokenInfo.Data["display_name"].(string); ok && username != "" {
		fmt.Printf("Authenticated as: %s\n", username)
	}

	policies, ok := tokenInfo.Data["policies"].([]interface{})
	if !ok {
		return
	}

	fmt.Print("Policies: ")
	for i, p := range policies {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(p)
	}
	fmt.Println()
}
