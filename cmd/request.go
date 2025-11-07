package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/internal/table"
	"github.com/gateplane-io/client-cli/internal/vault"
	// "github.com/gateplane-io/client-cli/internal/service"
	"github.com/gateplane-io/client-cli/pkg/models"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	// "github.com/gateplane-io/vault-plugins/pkg/models"
	"github.com/gateplane-io/vault-plugins/pkg/responses"
)

func requestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "request",
		Aliases: []string{"r", "req", "requests"},
		Short:   "Manage access requests",
		Long:    "Create and manage access requests for GatePlane gates",
	}

	cmd.AddCommand(
		requestCreateCmd(),
		// requestStatusCmd(),
		requestListCmd(),
		// requestCancelCmd(),
	)

	return cmd
}

func requestCreateCmd() *cobra.Command {
	var (
		reason      string
		interactive bool
	)

	cmd := &cobra.Command{
		Use:     "create [gate]",
		Aliases: []string{"c", "new", "add"},
		Short:   "Create a new access request",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			useInteractive := isInteractiveMode(interactive, len(args) > 0, reason != "")

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			var gate string

			if useInteractive {
				// provide empty gate array to discover all gates
				var noGates []*models.Gate
				gate, err = selectGateInteractively(client, noGates)
				if err != nil {
					return err
				}
			} else {
				gate, err = resolveGateFromArgs(args)
				if err != nil {
					return err
				}
			}

			if reason == "" {
				if useInteractive {
					reason, err = promptForReason()
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("reason is required. Use --reason flag or enable interactive mode")
				}
			}

			if err := client.CreateRequest(gate, reason); err != nil {
				return wrapError("create request", err)
			}

			printSuccessMessage("Request created successfully on gate: %s", gate)

			// Get the status immediately
			req, err := client.GetRequestStatus(gate)
			if err == nil && req != nil {
				fmt.Printf("Status: %s\n", req.Status)

				// Send notification if service is authenticated
				/*
				notificationService := service.NewService(client)
				if err := notificationService.SendNotification(service.NotificationRequest, gate, req.RequestID); err != nil {
					// Log but don't fail on notification errors
					fmt.Printf("Warning: failed to send notification: %v\n", err)
				}
				*/
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for access request")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	return cmd
}

func requestListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list [gate]",
		Aliases: []string{"ls", "l"},
		Short:   "List requests for specified gate or gate prefix",
		Long:    "List requests for a specific gate or all gates matching a prefix. Use 'auth/prefix' to list all gates starting with that prefix.",
		RunE: func(cmd *cobra.Command, args []string) error {

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			var requests []*responses.AccessRequestResponse
			var gateFilter string

			// Check if gate argument is provided
			if len(args) > 0 {
				gateFilter = args[0]
			}

			// Discover all gates first
			gates, err := client.DiscoverGates()
			if err != nil {
				return fmt.Errorf("failed to discover gates: %w", err)
			}

			// Filter gates based on the provided argument
			var targetGates []*models.Gate
			if gateFilter == "" {
				// No filter, use all gates
				targetGates = gates
			} else {
				// Check if it's an exact match first
				exactMatch := false
				for _, gate := range gates {
					if gate.Path == gateFilter {
						targetGates = append(targetGates, gate)
						exactMatch = true
						break
					}
				}

				// If no exact match, treat as prefix filter
				if !exactMatch {
					for _, gate := range gates {
						if strings.HasPrefix(gate.Path, gateFilter) {
							targetGates = append(targetGates, gate)
						}
					}
				}
			}

			// Get requests from filtered gates
			for _, gate := range targetGates {
				gateRequests, err := client.ListAllRequests(gate.Path)
				if err == nil && gateRequests != nil && len(gateRequests) > 0 {
					requests = append(requests, gateRequests...)
				}
			}

			format := getEffectiveOutputFormat()
			if format == OutputFormatJSON || format == OutputFormatYAML {
				return formatOutput(requests, format)
			}

			// Table format
			if len(requests) == 0 {
				fmt.Println("No requests found")
				return nil
			}

			rows := make([]table.Row, 0, len(requests))
			for _, req := range requests {
				rows = append(rows, table.Row{
					formatGateDisplay(req.Gate),
					req.User,
					formatRequestStatus(req),
					req.Reason,
					req.RequestID,
				})
			}

			table.RenderTable(table.TableOptions{
				Headers: []string{"Gate", "User", "Status", "Reason", "Request ID"},
				SortBy:  0, // Sort by Gate
				GroupBy: 0, // Group by Gate
			}, rows)

			return nil
		},
	}
}

// selectGateInteractively handles the interactive gate selection flow
func selectGateInteractively(client *vault.Client, gates []*models.Gate) (string, error) {
	var err error
	if len(gates) == 0 {
		gates, err = client.DiscoverGates()

		if err != nil {
			return "", wrapError("discover gates", err)
		}
	}

	if len(gates) == 0 {
		return "", fmt.Errorf("no gates discovered")
	}

	// Sort gates alphabetically by path
	sort.Slice(gates, func(i, j int) bool {
		return gates[i].Path < gates[j].Path
	})

	cfg := config.GetConfig()
	defaultGatePath := ""
	if cfg.Defaults.Gate != "" {
		defaultGatePath = config.ResolveGatePath(cfg.Defaults.Gate)
	}

	// Create gate selection items with nice display names and mark default
	gateItems := make([]string, len(gates))
	defaultIndex := 0
	for i, g := range gates {
		var displayName string
		if g.Alias != "" {
			displayName = fmt.Sprintf("%s (%s) [%s]", g.Alias, g.Path, g.Type)
		} else {
			displayName = fmt.Sprintf("%s [%s]", g.Path, g.Type)
		}

		// Mark default gate with cyan color and asterisk
		if g.Path == defaultGatePath {
			gateItems[i] = color.CyanString("%s *", displayName)
			defaultIndex = i
		} else {
			gateItems[i] = displayName
		}
	}

	prompt := promptui.Select{
		Label:             "Select gate",
		Items:             gateItems,
		CursorPos:         defaultIndex,
		Size:              10,
		StartInSearchMode: len(gates) > 10,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(gateItems[index]), strings.ToLower(input))
		},
	}

	selectedIndex, _, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("gate selection cancelled: %w", err)
	}

	return gates[selectedIndex].Path, nil
}

// promptForReason prompts the user to enter a reason for the access request
func promptForReason() (string, error) {
	validate := func(input string) error {
		if strings.TrimSpace(input) == "" {
			return fmt.Errorf("reason cannot be empty")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Reason for access",
		Validate: validate,
	}

	result, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("reason input cancelled: %w", err)
	}
	return result, nil
}

func requestCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cancel [gate]",
		Aliases: []string{"delete", "rm"},
		Short:   "Cancel your pending request on a gate",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gate, err := resolveGateFromArgs(args)
			if err != nil {
				return err
			}

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			path := fmt.Sprintf("%s/request", gate)
			_, err = client.VaultClient().Logical().Delete(path)
			if err != nil {
				return wrapError("cancel request", err)
			}

			printSuccessMessage("Request cancelled successfully")
			return nil
		},
	}

	return cmd
}
