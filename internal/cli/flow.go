package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// NewFlowCmd создаёт группу команд для управления flows.
func NewFlowCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flow",
		Short: "Manage flows",
	}

	cmd.AddCommand(
		newFlowListCmd(clientFn, outputFn),
		newFlowCreateCmd(clientFn, outputFn),
		newFlowShowCmd(clientFn, outputFn),
		newFlowUpdateCmd(clientFn, outputFn),
		newFlowDeleteCmd(clientFn, outputFn),
		newFlowVersionsCmd(clientFn, outputFn),
		newFlowPublishCmd(clientFn, outputFn),
	)

	return cmd
}

func newFlowListCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all flows",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			flows, err := client.ListFlows()
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "ACTIVE", "CREATED"}
			rows := make([][]string, len(flows))
			for i, f := range flows {
				rows[i] = []string{f.ID, f.Name, strconv.FormatBool(f.IsActive), f.CreatedAt}
			}

			out.Print(headers, rows, flows)
			return nil
		},
	}
}

func newFlowCreateCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			flow, err := client.CreateFlow(name)
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Flow created: %s", flow.ID))
			out.Print(
				[]string{"ID", "NAME", "ACTIVE", "CREATED"},
				[][]string{{flow.ID, flow.Name, strconv.FormatBool(flow.IsActive), flow.CreatedAt}},
				flow,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Flow name (required)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newFlowShowCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show flow details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			flow, err := client.GetFlow(args[0])
			if err != nil {
				return err
			}

			out.Print(
				[]string{"ID", "NAME", "ACTIVE", "CREATED"},
				[][]string{{flow.ID, flow.Name, strconv.FormatBool(flow.IsActive), flow.CreatedAt}},
				flow,
			)
			return nil
		},
	}
}

func newFlowUpdateCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var name string
	var active string

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			req := UpdateFlowRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("active") {
				b, err := strconv.ParseBool(active)
				if err != nil {
					return fmt.Errorf("invalid value for --active: %s", active)
				}
				req.IsActive = &b
			}

			flow, err := client.UpdateFlow(args[0], req)
			if err != nil {
				return err
			}

			out.Success("Flow updated")
			out.Print(
				[]string{"ID", "NAME", "ACTIVE", "CREATED"},
				[][]string{{flow.ID, flow.Name, strconv.FormatBool(flow.IsActive), flow.CreatedAt}},
				flow,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New flow name")
	cmd.Flags().StringVar(&active, "active", "", "Set active status (true/false)")

	return cmd
}

func newFlowDeleteCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			if err := client.DeleteFlow(args[0]); err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Flow deleted: %s", args[0]))
			return nil
		},
	}
}

func newFlowVersionsCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "versions FLOW_ID",
		Short: "List flow versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			versions, err := client.ListVersions(args[0])
			if err != nil {
				return err
			}

			headers := []string{"FLOW_ID", "VERSION", "CREATED"}
			rows := make([][]string, len(versions))
			for i, v := range versions {
				rows[i] = []string{v.FlowID, strconv.Itoa(v.Version), v.CreatedAt}
			}

			out.Print(headers, rows, versions)
			return nil
		},
	}
}

func newFlowPublishCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var specFile string

	cmd := &cobra.Command{
		Use:   "publish FLOW_ID",
		Short: "Publish a new flow version from spec file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			data, err := os.ReadFile(specFile)
			if err != nil {
				return fmt.Errorf("failed to read spec file: %w", err)
			}

			// Валидируем что это валидный JSON
			if !json.Valid(data) {
				return fmt.Errorf("spec file is not valid JSON")
			}

			version, err := client.CreateVersion(args[0], json.RawMessage(data))
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Version %d published for flow %s", version.Version, version.FlowID))
			out.Print(
				[]string{"FLOW_ID", "VERSION", "CREATED"},
				[][]string{{version.FlowID, strconv.Itoa(version.Version), version.CreatedAt}},
				version,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&specFile, "spec-file", "", "Path to spec JSON file (required)")
	cmd.MarkFlagRequired("spec-file")

	return cmd
}
