package main

import (
	"fmt"
	"strings"

	"github.com/gateplane-io/client-cli/internal/config"
	// "github.com/gateplane-io/client-cli/internal/service"
	"github.com/gateplane-io/client-cli/pkg/models"

	base "github.com/gateplane-io/vault-plugins/pkg/models"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func approveCmd() *cobra.Command {
	var interactive bool

	cmd := &cobra.Command{
		Use:     "approve [gate] [requestor-id]",
		Aliases: []string{"a", "app"},
		Short:   "Approve an access request",
		Long:    "Approve access request using gate and request ID. If no arguments provided and running in TTY, enters interactive mode.",
		Args:    cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			useInteractive := isInteractiveMode(interactive, len(args) > 0, false)

			if useInteractive {
				return runInteractiveApprove()
			}

			// Non-interactive mode - require both arguments
			if len(args) != 2 {
				return fmt.Errorf("both gate and requestor-id are required in non-interactive mode")
			}

			gate := config.ResolveGatePath(args[0])
			requestID := args[1]
			return approveRequest(cmd, requestID, gate)
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	return cmd
}

func runInteractiveApprove() error {
	client, err := createVaultClient()
	if err != nil {
		return wrapError("create vault client", err)
	}

	currentUser, err := client.GetSelf()
	if err != nil {
		return wrapError("get current user", err)
	}

	gates, err := client.DiscoverGates()
	if err != nil {
		return wrapError("discover gates", err)
	}

	if len(gates) == 0 {
		return fmt.Errorf("no gates discovered")
	}

	// Collect all pending requests across all gates
	var allRequests []*models.Request
	for _, gate := range gates {
		// Use ListAllRequests and filter for pending ones
		gateRequests, err := client.ListAllRequestsForGate(gate.Path)
		if err != nil {
			// Continue to next gate if this one fails
			continue
		}
		// Filter for requests that can be approved by current user:
		// - Must be pending (not approved, denied, expired, or active)
		// - Must not be from the current user (can't approve own requests)
		// - Current user must not have already voted on it
		for _, req := range gateRequests {
			if req.Status == base.Pending &&
				req.OwnerID != currentUser.Entity.ID &&
				!req.HaveApproved {
				allRequests = append(allRequests, req)
			}
		}
	}

	if len(allRequests) == 0 {
		printSuccessMessage("All caught up! No pending requests require your approval.")
		fmt.Println("  • Requests already approved by you are not shown")
		fmt.Println("  • Your own requests are not shown")
		fmt.Println("  • Only requests in 'pending' status are shown")
		return nil
	}

	// Create display items for requests
	requestItems := make([]string, len(allRequests))
	for i, req := range allRequests {
		requestItems[i] = fmt.Sprintf("[%s] - Approvals: %d/%d (ID: %.8s) - %s",
			req.Gate.Path,
			req.NumOfApprovals,
			req.RequiredApprovals,
			req.OwnerID,
			req.Justification,
		)
	}

	// Select request to approve
	prompt := promptui.Select{
		Label:             "Select request to approve",
		Items:             requestItems,
		Size:              10,
		StartInSearchMode: len(allRequests) > 10,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(requestItems[index]), strings.ToLower(input))
		},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}?",
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "✓ {{ . | green }}",
		},
	}

	selectedIndex, _, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("request selection cancelled: %w", err)
	}

	selectedRequest := allRequests[selectedIndex]

	// Confirm approval
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Approve request from %s on gate '%s'", currentUser.Entity.ID, selectedRequest.Gate.Path),
		IsConfirm: true,
	}

	_, err = confirmPrompt.Run()
	if err != nil {
		return fmt.Errorf("approval cancelled")
	}

	// Approve the request using the request ID
	return approveRequest(nil, selectedRequest.OwnerID, selectedRequest.Gate.Path)
}

func approveRequest(cmd *cobra.Command, requestID string, gate string) error {
	client, err := createVaultClient()
	if err != nil {
		return wrapError("create vault client", err)
	}

	if err := client.ApproveRequest(gate, requestID); err != nil {
		return wrapError("approve request", err)
	}

	printSuccessMessage("Approved request %s on gate: %s", requestID, gate)

	// Send notification if service is authenticated
	// notificationService := service.NewService(client)
	// if err := notificationService.SendNotification(service.NotificationApprove, gate, requestID); err != nil {
	// 	// Log but don't fail on notification errors
	// 	fmt.Printf("Warning: failed to send notification: %v\n", err)
	// }

	return nil
}
