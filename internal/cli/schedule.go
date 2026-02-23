package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// NewScheduleCmd создаёт группу команд для управления schedules.
func NewScheduleCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage schedules",
	}

	cmd.AddCommand(
		newScheduleListCmd(clientFn, outputFn),
		newScheduleCreateCmd(clientFn, outputFn),
		newScheduleShowCmd(clientFn, outputFn),
		newScheduleUpdateCmd(clientFn, outputFn),
		newScheduleDeleteCmd(clientFn, outputFn),
		newScheduleEnableCmd(clientFn, outputFn),
		newScheduleDisableCmd(clientFn, outputFn),
	)

	return cmd
}

func newScheduleListCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var flowID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List schedules",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			schedules, err := client.ListSchedules(flowID)
			if err != nil {
				return err
			}

			headers := []string{"ID", "FLOW_ID", "NAME", "CRON", "INTERVAL", "ENABLED", "NEXT_DUE"}
			rows := make([][]string, len(schedules))
			for i, s := range schedules {
				interval := ""
				if s.IntervalSec > 0 {
					interval = strconv.Itoa(s.IntervalSec) + "s"
				}
				rows[i] = []string{
					s.ID, s.FlowID, s.Name, s.CronExpr, interval,
					strconv.FormatBool(s.Enabled), s.NextDueAt,
				}
			}

			out.Print(headers, rows, schedules)
			return nil
		},
	}

	cmd.Flags().StringVar(&flowID, "flow-id", "", "Filter by flow ID")

	return cmd
}

func newScheduleCreateCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var name string
	var cronExpr string
	var intervalSec int
	var timezone string
	var inputs []string

	cmd := &cobra.Command{
		Use:   "create FLOW_ID",
		Short: "Create a schedule for a flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			req := CreateScheduleRequest{
				Name:        name,
				CronExpr:    cronExpr,
				IntervalSec: intervalSec,
				Timezone:    timezone,
				Enabled:     true,
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

			schedule, err := client.CreateSchedule(args[0], req)
			if err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Schedule created: %s", schedule.ID))
			out.Print(
				[]string{"ID", "FLOW_ID", "NAME", "CRON", "INTERVAL", "ENABLED"},
				[][]string{{
					schedule.ID, schedule.FlowID, schedule.Name, schedule.CronExpr,
					formatInterval(schedule.IntervalSec), strconv.FormatBool(schedule.Enabled),
				}},
				schedule,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Schedule name (required)")
	cmd.Flags().StringVar(&cronExpr, "cron", "", "Cron expression (e.g. '0 * * * *')")
	cmd.Flags().IntVar(&intervalSec, "interval", 0, "Interval in seconds")
	cmd.Flags().StringVar(&timezone, "timezone", "", "Timezone (e.g. 'Europe/Moscow')")
	cmd.Flags().StringSliceVar(&inputs, "input", nil, "Input values as KEY=VALUE (repeatable)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newScheduleShowCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show schedule details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			schedule, err := client.GetSchedule(args[0])
			if err != nil {
				return err
			}

			out.Print(
				[]string{"ID", "FLOW_ID", "NAME", "CRON", "INTERVAL", "TIMEZONE", "ENABLED", "NEXT_DUE"},
				[][]string{{
					schedule.ID, schedule.FlowID, schedule.Name, schedule.CronExpr,
					formatInterval(schedule.IntervalSec), schedule.Timezone,
					strconv.FormatBool(schedule.Enabled), schedule.NextDueAt,
				}},
				schedule,
			)
			return nil
		},
	}
}

func newScheduleUpdateCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	var name string
	var cronExpr string
	var intervalSec int
	var timezone string

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			req := UpdateScheduleRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("cron") {
				req.CronExpr = &cronExpr
			}
			if cmd.Flags().Changed("interval") {
				req.IntervalSec = &intervalSec
			}
			if cmd.Flags().Changed("timezone") {
				req.Timezone = &timezone
			}

			schedule, err := client.UpdateSchedule(args[0], req)
			if err != nil {
				return err
			}

			out.Success("Schedule updated")
			out.Print(
				[]string{"ID", "FLOW_ID", "NAME", "CRON", "INTERVAL", "ENABLED"},
				[][]string{{
					schedule.ID, schedule.FlowID, schedule.Name, schedule.CronExpr,
					formatInterval(schedule.IntervalSec), strconv.FormatBool(schedule.Enabled),
				}},
				schedule,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New schedule name")
	cmd.Flags().StringVar(&cronExpr, "cron", "", "New cron expression")
	cmd.Flags().IntVar(&intervalSec, "interval", 0, "New interval in seconds")
	cmd.Flags().StringVar(&timezone, "timezone", "", "New timezone")

	return cmd
}

func newScheduleDeleteCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			if err := client.DeleteSchedule(args[0]); err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Schedule deleted: %s", args[0]))
			return nil
		},
	}
}

func newScheduleEnableCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "enable ID",
		Short: "Enable a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			if _, err := client.EnableSchedule(args[0]); err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Schedule enabled: %s", args[0]))
			return nil
		},
	}
}

func newScheduleDisableCmd(clientFn func() *Client, outputFn func() *Output) *cobra.Command {
	return &cobra.Command{
		Use:   "disable ID",
		Short: "Disable a schedule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := clientFn()
			out := outputFn()

			if _, err := client.DisableSchedule(args[0]); err != nil {
				return err
			}

			out.Success(fmt.Sprintf("Schedule disabled: %s", args[0]))
			return nil
		},
	}
}

func formatInterval(sec int) string {
	if sec <= 0 {
		return ""
	}
	return strconv.Itoa(sec) + "s"
}
