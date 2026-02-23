package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// NewRunCmd создаёт группу команд для управления runs.
func NewRunCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage runs",
	}

	cmd.AddCommand(
		newRunListCmd(clientFn, outputFn),
		newRunStartCmd(clientFn, outputFn),
		newRunShowCmd(clientFn, outputFn),
		newRunCancelCmd(clientFn, outputFn),
		newRunTasksCmd(clientFn, outputFn),
	)

	return cmd
}

func newRunListCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var flowID string
	var status string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			runs, err := client.ListRuns(ListRunsOpts{
				FlowID: flowID,
				Status: status,
				Limit:  limit,
			})
			if err != nil {
				return err
			}

			headers := []string{"ID", "FLOW_ID", "VERSION", "STATUS", "CREATED"}
			rows := make([][]string, len(runs))
			for i, r := range runs {
				rows[i] = []string{r.ID, r.FlowID, strconv.Itoa(r.Version), r.Status, r.CreatedAt}
			}

			out.Print(headers, rows, runs)
			return nil
		},
	}

	cmd.Flags().StringVar(&flowID, "flow-id", "", "Filter by flow ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (PENDING, RUNNING, SUCCEEDED, FAILED, CANCELLED)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results")

	return cmd
}

func newRunStartCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var version int
	var inputs []string
	var sandbox bool

	cmd := &cobra.Command{
		Use:   "start FLOW_ID",
		Short: "Start a new run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			req := CreateRunRequest{
				IsSandbox: sandbox,
			}

			if cmd.Flags().Changed("version") {
				req.Version = &version
			}

			if len(inputs) > 0 {
				req.Inputs = make(map[string]any)
				for _, kv := range inputs {
					parts := strings.SplitN(kv, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid input format %q, expected KEY=VALUE", kv)
					}
					req.Inputs[parts[0]] = parts[1]
				}
			}

			run, err := client.CreateRun(args[0], req)
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Run started: %s", run.ID))
			out.Print(
				[]string{"ID", "FLOW_ID", "VERSION", "STATUS", "CREATED"},
				[][]string{{run.ID, run.FlowID, strconv.Itoa(run.Version), run.Status, run.CreatedAt}},
				run,
			)
			return nil
		},
	}

	cmd.Flags().IntVar(&version, "version", 0, "Flow version (latest if not specified)")
	cmd.Flags().StringSliceVar(&inputs, "input", nil, "Input values as KEY=VALUE (repeatable)")
	cmd.Flags().BoolVar(&sandbox, "sandbox", false, "Run in sandbox mode")

	return cmd
}

func newRunShowCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show run details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			run, err := client.GetRun(args[0])
			if err != nil {
				return err
			}

			out.Print(
				[]string{"ID", "FLOW_ID", "VERSION", "STATUS", "ERROR", "CREATED"},
				[][]string{{run.ID, run.FlowID, strconv.Itoa(run.Version), run.Status, run.Error, run.CreatedAt}},
				run,
			)
			return nil
		},
	}
}

func newRunCancelCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel ID",
		Short: "Cancel a running run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			run, err := client.CancelRun(args[0])
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Run cancelled: %s", run.ID))
			return nil
		},
	}
}

func newRunTasksCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "tasks RUN_ID",
		Short: "List tasks in a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			tasks, err := client.ListTasks(args[0])
			if err != nil {
				return err
			}

			headers := []string{"ID", "STEP_ID", "TYPE", "STATUS", "ATTEMPT", "ERROR"}
			rows := make([][]string, len(tasks))
			for i, t := range tasks {
				rows[i] = []string{t.ID, t.StepID, t.Type, t.Status, strconv.Itoa(t.Attempt), t.Error}
			}

			out.Print(headers, rows, tasks)
			return nil
		},
	}
}
