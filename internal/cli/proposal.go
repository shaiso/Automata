package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewProposalCmd создаёт группу команд для управления proposals.
func NewProposalCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proposal",
		Short: "Manage proposals (PR-workflow)",
	}

	cmd.AddCommand(
		newProposalListCmd(clientFn, outputFn),
		newProposalCreateCmd(clientFn, outputFn),
		newProposalShowCmd(clientFn, outputFn),
		newProposalUpdateCmd(clientFn, outputFn),
		newProposalDeleteCmd(clientFn, outputFn),
		newProposalSubmitCmd(clientFn, outputFn),
		newProposalApproveCmd(clientFn, outputFn),
		newProposalRejectCmd(clientFn, outputFn),
		newProposalApplyCmd(clientFn, outputFn),
		newProposalSandboxCmd(clientFn, outputFn),
	)

	return cmd
}

func newProposalListCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var flowID string
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proposals",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			proposals, err := client.ListProposals(ListProposalsOpts{
				FlowID: flowID,
				Status: status,
			})
			if err != nil {
				return err
			}

			headers := []string{"ID", "FLOW_ID", "TITLE", "STATUS", "CREATED_BY", "CREATED"}
			rows := make([][]string, len(proposals))
			for i, p := range proposals {
				rows[i] = []string{p.ID, p.FlowID, p.Title, p.Status, p.CreatedBy, p.CreatedAt}
			}

			out.Print(headers, rows, proposals)
			return nil
		},
	}

	cmd.Flags().StringVar(&flowID, "flow-id", "", "Filter by flow ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (DRAFT, PENDING_REVIEW, APPROVED, REJECTED, APPLIED)")

	return cmd
}

func newProposalCreateCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var title string
	var description string
	var createdBy string
	var specFile string

	cmd := &cobra.Command{
		Use:   "create FLOW_ID",
		Short: "Create a new proposal for a flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			data, err := os.ReadFile(specFile)
			if err != nil {
				return fmt.Errorf("failed to read spec file: %w", err)
			}

			if !json.Valid(data) {
				return fmt.Errorf("spec file is not valid JSON")
			}

			req := CreateProposalRequest{
				Title:       title,
				Description: description,
				CreatedBy:   createdBy,
				Spec:        json.RawMessage(data),
			}

			proposal, err := client.CreateProposal(args[0], req)
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Proposal created: %s", proposal.ID))
			out.Print(
				[]string{"ID", "FLOW_ID", "TITLE", "STATUS", "CREATED"},
				[][]string{{proposal.ID, proposal.FlowID, proposal.Title, proposal.Status, proposal.CreatedAt}},
				proposal,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Proposal title (required)")
	cmd.Flags().StringVar(&description, "description", "", "Proposal description")
	cmd.Flags().StringVar(&createdBy, "created-by", "", "Author name")
	cmd.Flags().StringVar(&specFile, "spec-file", "", "Path to spec JSON file (required)")
	cmd.MarkFlagRequired("title")
	cmd.MarkFlagRequired("spec-file")

	return cmd
}

func newProposalShowCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show proposal details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			proposal, err := client.GetProposal(args[0])
			if err != nil {
				return err
			}

			out.Print(
				[]string{"ID", "FLOW_ID", "TITLE", "STATUS", "CREATED_BY", "REVIEWED_BY", "CREATED"},
				[][]string{{
					proposal.ID, proposal.FlowID, proposal.Title, proposal.Status,
					proposal.CreatedBy, proposal.ReviewedBy, proposal.CreatedAt,
				}},
				proposal,
			)
			return nil
		},
	}
}

func newProposalUpdateCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var title string
	var description string
	var specFile string

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a proposal (DRAFT only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			req := UpdateProposalRequestCLI{}
			if cmd.Flags().Changed("title") {
				req.Title = &title
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("spec-file") {
				data, err := os.ReadFile(specFile)
				if err != nil {
					return fmt.Errorf("failed to read spec file: %w", err)
				}
				if !json.Valid(data) {
					return fmt.Errorf("spec file is not valid JSON")
				}
				req.Spec = json.RawMessage(data)
			}

			proposal, err := client.UpdateProposal(args[0], req)
			if err != nil {
				return err
			}

			out.Success("Proposal updated")
			out.Print(
				[]string{"ID", "FLOW_ID", "TITLE", "STATUS", "CREATED"},
				[][]string{{proposal.ID, proposal.FlowID, proposal.Title, proposal.Status, proposal.CreatedAt}},
				proposal,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	cmd.Flags().StringVar(&specFile, "spec-file", "", "Path to new spec JSON file")

	return cmd
}

func newProposalDeleteCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a proposal (DRAFT only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			if err := client.DeleteProposal(args[0]); err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Proposal deleted: %s", args[0]))
			return nil
		},
	}
}

func newProposalSubmitCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "submit ID",
		Short: "Submit proposal for review (DRAFT -> PENDING_REVIEW)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			proposal, err := client.SubmitProposal(args[0])
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Proposal submitted for review: %s", proposal.ID))
			out.Print(
				[]string{"ID", "TITLE", "STATUS"},
				[][]string{{proposal.ID, proposal.Title, proposal.Status}},
				proposal,
			)
			return nil
		},
	}
}

func newProposalApproveCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var reviewer string
	var comment string

	cmd := &cobra.Command{
		Use:   "approve ID",
		Short: "Approve a proposal (PENDING_REVIEW -> APPROVED)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			proposal, err := client.ApproveProposal(args[0], ReviewRequest{
				Reviewer: reviewer,
				Comment:  comment,
			})
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Proposal approved: %s", proposal.ID))
			out.Print(
				[]string{"ID", "TITLE", "STATUS", "REVIEWED_BY"},
				[][]string{{proposal.ID, proposal.Title, proposal.Status, proposal.ReviewedBy}},
				proposal,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&reviewer, "reviewer", "", "Reviewer name (required)")
	cmd.Flags().StringVar(&comment, "comment", "", "Review comment")
	cmd.MarkFlagRequired("reviewer")

	return cmd
}

func newProposalRejectCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var reviewer string
	var comment string

	cmd := &cobra.Command{
		Use:   "reject ID",
		Short: "Reject a proposal (PENDING_REVIEW -> REJECTED)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			proposal, err := client.RejectProposal(args[0], ReviewRequest{
				Reviewer: reviewer,
				Comment:  comment,
			})
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Proposal rejected: %s", proposal.ID))
			out.Print(
				[]string{"ID", "TITLE", "STATUS", "REVIEWED_BY"},
				[][]string{{proposal.ID, proposal.Title, proposal.Status, proposal.ReviewedBy}},
				proposal,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&reviewer, "reviewer", "", "Reviewer name (required)")
	cmd.Flags().StringVar(&comment, "comment", "", "Review comment")
	cmd.MarkFlagRequired("reviewer")

	return cmd
}

func newProposalApplyCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "apply ID",
		Short: "Apply a proposal — create new flow version (APPROVED -> APPLIED)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			proposal, err := client.ApplyProposal(args[0])
			if err != nil {
				return err
			}

			versionStr := ""
			if proposal.AppliedVersion != nil {
				versionStr = fmt.Sprintf("%d", *proposal.AppliedVersion)
			}

			out.Success(fmt.Sprintf("Proposal applied: %s (version %s)", proposal.ID, versionStr))
			out.Print(
				[]string{"ID", "FLOW_ID", "TITLE", "STATUS", "APPLIED_VERSION"},
				[][]string{{proposal.ID, proposal.FlowID, proposal.Title, proposal.Status, versionStr}},
				proposal,
			)
			return nil
		},
	}
}

func newProposalSandboxCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "sandbox ID",
		Short: "Run sandbox test for a proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			proposal, err := client.RunProposalSandbox(args[0])
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Sandbox run started for proposal: %s (run_id: %s)", proposal.ID, proposal.SandboxRunID))
			out.Print(
				[]string{"ID", "TITLE", "STATUS", "SANDBOX_RUN_ID"},
				[][]string{{proposal.ID, proposal.Title, proposal.Status, proposal.SandboxRunID}},
				proposal,
			)
			return nil
		},
	}
}
