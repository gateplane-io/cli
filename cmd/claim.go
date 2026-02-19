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

	"github.com/fatih/color"

	"github.com/gateplane-io/client-cli/internal/service"
	"github.com/gateplane-io/client-cli/pkg/models"

	base "github.com/gateplane-io/vault-plugins/pkg/models"

	"github.com/spf13/cobra"
)

func claimCmd() *cobra.Command {
	var (
		interactive bool
		gate        string
	)

	cmd := &cobra.Command{
		Use:     "claim [gate]",
		Aliases: []string{"c"},
		Short:   "Claim approved access",
		Long:    "Claim approved access",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			useInteractive := isInteractiveMode(interactive, len(args) > 0, gate != "")

			// var claimableGates []*models.Gate

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			svcClient, err := createServiceClient()
			if err != nil {
				fmt.Println("Not authenticated with GatePlane Services (using Community Edition features)")
				svcClient = nil
			}

			if useInteractive {
				// Discover all gates first
				gates, err := client.DiscoverGates()
				if err != nil {
					return wrapError("discover gates", err)
				}

				// Get claimable requests from gates
				var gateRequest *models.Request
				var claimableGates []*models.Gate
				for _, gate := range gates {
					gateRequest, err = client.GetRequestStatus(gate.Path)
					if err == nil && gateRequest != nil && gateRequest.Status == base.Approved {
						claimableGates = append(claimableGates, gate)
					}
				}

				// No Request can be claimed, do not proceed.
				if len(claimableGates) == 0 {
					color.New(color.Bold).Println("No Claimable Requests available.")
					return nil
				}

				gate, err = selectGateInteractively(client, claimableGates)
				if err != nil {
					return err
				}
			} else {
				gate, err = resolveGateFromArgs(args)
				if err != nil {
					return err
				}
			}

			req, err := client.GetRequestStatus(gate)
			if err != nil {
				return wrapError("get request status", err)
			}

			if req.Status != base.Approved {
				return wrapError("claim access", fmt.Errorf("request is not approved (status: %s)", req.Status))
			}

			claimResponse, err := client.ClaimAccess(gate)
			if err != nil {
				return wrapError("claim access", err)
			}

			// Send notification if service is authenticated
			if err := sendNotificationWithRetry(svcClient, client, req, gate, service.Claim); err != nil {
				return wrapError("send notification", err)
			}

			format := getEffectiveOutputFormat()
			switch format {
			case OutputFormatJSON, OutputFormatYAML:
				return formatOutput(claimResponse, format)

			default: // table
				printSuccessMessage("Access claimed successfully on gate: %s", gate)
			}

			accessStruct, err := client.GetPolicyGateAccessStruct(gate)
			if err == nil {
				fmt.Println("Claimed Access:")
				renderAccessTable(*accessStruct)
			} else {
				fmt.Printf("Warning: failed to get gate access struct for notification: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	return cmd
}
