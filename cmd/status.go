package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/gateplane-io/client-cli/internal/table"
	"github.com/gateplane-io/client-cli/pkg/models"
	"github.com/spf13/cobra"

	base "github.com/gateplane-io/vault-plugins/pkg/models"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"s", "st", "dash", "dashboard"},
		Short:   "Show dashboard of all active requests and pending approvals",
		RunE: func(cmd *cobra.Command, args []string) error {

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			currentUser, err := client.GetSelf()
			if err != nil {
				return wrapError("get entity name", err)
			}

			// Discover all gates
			gates, err := client.DiscoverGates()
			if err != nil {
				return wrapError("discover gates", err)
			}

			// Collect your requests
			var myRequests []*models.Request
			var pendingApprovals []*models.Request

			for _, gate := range gates {
				// Check for your own requests
				ownReq, err := client.GetRequestStatus(gate.Path)
				if err == nil && ownReq != nil {
					myRequests = append(myRequests, ownReq)
				}

				requests, err := client.ListAllRequestsForGate(gate.Path)
				if err != nil {
					// We are not "approvers" for this gate,
					// and cannot see requests from others
					continue
				}

				for _, req := range requests {
					// Check for pending approvals
					if req.Status == base.Pending && req.OwnerID != currentUser.Entity.ID {
						pendingApprovals = append(pendingApprovals, req)
					}
				}
			}

			// Display your requests
			fmt.Println(color.CyanString("Your Active Requests:"))
			if len(myRequests) == 0 {
				fmt.Println("  No active requests")
			} else {
				rows := make([]table.Row, 0, len(myRequests))
				for _, req := range myRequests {
					// Format gate path with alias if available
					gatePath := req.Gate.Path
					for _, g := range gates {
						if g.Path == req.Gate.Path && g.Alias != "" {
							gatePath = fmt.Sprintf("%s (%s)", g.Alias, req.Gate.Path)
							break
						}
					}

					rows = append(rows, table.Row{
						formatGateDisplay(gatePath),
						formatRequestStatus(req.Status),
						req.Justification,
					})
				}

				table.RenderTable(table.TableOptions{
					Headers: []string{"Gate", "Status", "Justification"},
					SortBy:  0,  // Sort by Gate
					GroupBy: -1, // No grouping for own requests
				}, rows)
			}

			// Display pending approvals
			fmt.Println("\n" + color.CyanString("Pending Approvals (for you to approve):"))
			if len(pendingApprovals) == 0 {
				fmt.Println("  No pending approvals")
			} else {
				rows := make([]table.Row, 0, len(pendingApprovals))
				for _, req := range pendingApprovals {
					// Format gate path with alias if available
					gatePath := req.Gate.Path
					for _, g := range gates {
						if g.Path == req.Gate.Path && g.Alias != "" {
							gatePath = fmt.Sprintf("%s (%s)", g.Alias, req.Gate)
							break
						}
					}

					rows = append(rows, table.Row{
						formatGateDisplay(gatePath),
						req.OwnerID,
						req.Justification,
					})
				}

				table.RenderTable(table.TableOptions{
					Headers: []string{"Gate", "Requestor ID", "Justification"},
					SortBy:  0, // Sort by Gate
					GroupBy: 0, // Group by Gate
				}, rows)

				if len(pendingApprovals) > 0 {
					fmt.Println("\nTo approve a request:")
					fmt.Println("  gateplane approve [gate-path] [requestor-id]")
					fmt.Println("\nor interactively:")
					fmt.Println("  gateplane approve --interactive")
				}
			}

			// Display claimable requests
			fmt.Println("\n" + color.CyanString("Your Claimable Requests:"))
			claimableRequests := make([]*models.Request, 0)
			for _, req := range myRequests {
				if req.Status == base.Approved {
					claimableRequests = append(claimableRequests, req)
				}
			}

			if len(claimableRequests) == 0 {
				fmt.Println("  No claimable requests")
			} else {
				for _, req := range claimableRequests {
					// Get gate name or alias
					gateName := req.Gate.Path
					for _, g := range gates {
						if g.Path == req.Gate.Path && g.Alias != "" {
							gateName = g.Alias
							break
						}
					}

					// Format: - <gate name>: <request id> # <reason>
					fmt.Printf("- %s: %s %s\n",
						gateName,
						color.New(color.Bold).Sprint(req.OwnerID),
						color.New(color.Faint).Sprint("# "+req.Justification))

					// Show claim command
					if _, err := color.New(color.Bold, color.FgGreen).Printf("  gateplane claim %s\n", gateName); err != nil {
						return wrapError("print claim command", err)
					}
				}
			}

			return nil
		},
	}
}
