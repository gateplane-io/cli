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

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Version information set by ldflags during build
	Version    = "dev"
	CommitHash = "unknown"
	BuildDate  = "unknown"
	DebugBuild = false

	vaultToken   string
	vaultAddr    string
	outputFormat string

	rootCmd = &cobra.Command{
		Use:   "gateplane",
		Short: "CLI for GatePlane - Just-In-Time Access Management",
		Long: `GatePlane CLI provides command-line access to GatePlane gates for
requesting, approving, and claiming time-limited access to protected resources.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if err := config.Init(); err != nil {
				fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
			}
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&vaultToken, "vault-token", "t", "", "Vault token for authentication")
	rootCmd.PersistentFlags().StringVarP(&vaultAddr, "vault-addr", "a", "", "Vault server address")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format (table, json, yaml)")

	rootCmd.AddCommand(
		authCmd(),
		configCmd(),
		gatesCmd(),
		requestCmd(),
		approveCmd(),
		claimCmd(),
		statusCmd(),
		versionCmd(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("GatePlane CLI %s\n", Version)
			fmt.Printf("Commit: %s\n", CommitHash)
			fmt.Printf("Built: %s\n", BuildDate)
		},
	}
}
