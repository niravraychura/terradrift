package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/logger"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/scanner"
	"github.com/spf13/cobra"
)

const (
	exitCodeOK            = 0
	exitCodeFailure       = 1
	exitCodeDriftDetected = 2
)

type outputFormat string

const (
	outputFormatTable outputFormat = "table"
	outputFormatJSON  outputFormat = "json"
)

func main() {
	if err := newRootCommand(os.Stdout, os.Stderr).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(exitCodeFailure)
	}
	os.Exit(exitCodeOK)
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:           "terradrift",
		Short:         "Self-hosted Terraform drift detection",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level, err := logger.ParseLevel(logLevel)
			if err != nil {
				return err
			}
			slog.SetDefault(logger.New(stderr, level))
			return nil
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	cmd.AddCommand(newScanCommand(stdout))
	return cmd
}

func newScanCommand(stdout io.Writer) *cobra.Command {
	var directory string
	var format string
	var timeout time.Duration
	var redactPaths bool

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Validate a Terraform directory for drift scanning",
		Example: `  terradrift scan
  terradrift scan --directory ./terraform/prod
  terradrift scan -d ./terraform/prod --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedFormat, err := parseOutputFormat(format)
			if err != nil {
				return err
			}

			result, err := scanner.Scan(cmd.Context(), scanner.Options{
				Directory: directory,
				Timeout:   timeout,
			})
			if err != nil {
				return err
			}

			scanReport := result.Report
			if redactPaths {
				scanReport.Directory = "[REDACTED]"
			}
			return writeScanReport(stdout, scanReport, parsedFormat)
		},
	}
	cmd.Flags().StringVarP(&directory, "directory", "d", ".", "Terraform directory to scan")
	cmd.Flags().StringVarP(&format, "output", "o", string(outputFormatTable), "output format: table, json")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultTimeout, "maximum scan duration")
	cmd.Flags().BoolVar(&redactPaths, "redact-paths", false, "redact local filesystem paths from scan output")
	return cmd
}

func parseOutputFormat(format string) (outputFormat, error) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	switch outputFormat(normalized) {
	case outputFormatTable, outputFormatJSON:
		return outputFormat(normalized), nil
	default:
		return "", fmt.Errorf("unsupported output format %q; supported values: table, json", format)
	}
}

func writeScanReport(stdout io.Writer, scanReport report.DriftReport, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(scanReport); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	case outputFormatTable:
		if _, err := fmt.Fprintln(stdout, "TerraDrift scan initialized"); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Status: %s\n", scanReport.Status); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Terraform directory: %s\n", scanReport.Directory); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Resources checked: %d\n", scanReport.TotalResourcesChecked); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Changed resources: %d\n", scanReport.TotalChangedResources); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q; supported values: table, json", format)
	}
}
