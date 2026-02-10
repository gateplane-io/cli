package main

import (
	"fmt"

	"github.com/fatih/color"
	// "github.com/gateplane-io/client-cli/internal/service"

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
		Long:    "Claim approved access (TODO)",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			useInteractive := isInteractiveMode(interactive, len(args) > 0, gate != "")

			// var claimableGates []*models.Gate

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			if useInteractive {
				// Discover all gates first
				gates, err := client.DiscoverGates()
				if err != nil {
					return fmt.Errorf("failed to discover gates: %w", err)
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
				return fmt.Errorf("request is not approved (status: %s)", req.Status)
			}

			claimResponse, err := client.ClaimAccess(gate)
			if err != nil {
				return wrapError("claim access", err)
			}

			// // Send notification if service is authenticated
			// notificationService := service.NewService(client)
			// if err := notificationService.SendNotification(service.NotificationClaim, gate, req.RequestID); err != nil {
			// 	// Log but don't fail on notification errors
			// 	fmt.Printf("Warning: failed to send notification: %v\n", err)
			// }

			format := getEffectiveOutputFormat()
			switch format {
			case OutputFormatJSON, OutputFormatYAML:
				return formatOutput(claimResponse, format)

			default: // table
				printSuccessMessage("Access claimed successfully on gate: %s", gate)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	return cmd
}
