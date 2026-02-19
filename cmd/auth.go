// Copyright (C) 2026 Ioannis Torakis <john.torakis@gmail.com>
// SPDX-License-Identifier: Elastic-2.0
//
// Licensed under the Elastic License 2.0.
// You may obtain a copy of the license at:
// https://www.elastic.co/licensing/elastic-license
//
// Use, modification, and redistribution permitted under the terms of the license,
// except for providing this software as a commercial service or product.

package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/gateplane-io/client-cli/internal/config"
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
		inputAddr  string
		namespace  string
		inputToken string
	)

	cmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"signin"},
		Short:   "Authenticate with Vault",
		Long:    "Authenticate with Vault/OpenBao using a token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			// Get vault address
			if inputAddr == "" {
				inputAddr = cfg.Vault.Address
				if inputAddr == "" {
					if addr := os.Getenv("VAULT_ADDR"); addr != "" {
						inputAddr = addr
					} else {
						fmt.Print("Enter Vault address: ")
						if _, err := fmt.Scanln(&inputAddr); err != nil {
							return wrapError("read vault address", err)
						}
					}
				}
			}

			// Token-based authentication
			if inputToken == "" {
				fmt.Print("Enter Vault token: ")
				tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return wrapError("read token", err)
				}
				fmt.Println()
				inputToken = string(tokenBytes)
			}

			// Update global config for client creation
			vaultAddr = inputAddr
			vaultToken = inputToken
			if namespace != "" {
				cfg.Vault.Namespace = namespace
			}

			// Test connection
			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			// Try to get token info to verify auth
			tokenInfo, err := client.VaultClient().Auth().Token().LookupSelf()
			if err != nil {
				return wrapError("authentication failed", err)
			}

			// Save config
			cfg.Vault.Address = vaultAddr
			cfg.Vault.Token = vaultToken
			if namespace != "" {
				cfg.Vault.Namespace = namespace
			}
			if err := config.SaveConfig(); err != nil {
				return wrapError("save config", err)
			}

			printAuthSuccessMessage(tokenInfo)

			return nil
		},
	}

	cmd.Flags().StringVar(&inputAddr, "address", "", "Vault address")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Vault namespace")
	cmd.Flags().StringVar(&inputToken, "token", "", "Vault token (use with caution)")

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
				return wrapError("save config", err)
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
