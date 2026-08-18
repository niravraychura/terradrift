package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/config"
	"github.com/niravraychura/terradrift/internal/dashboard"
	"github.com/niravraychura/terradrift/internal/history"
	"github.com/niravraychura/terradrift/internal/ioutil"
	"github.com/niravraychura/terradrift/internal/redact"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/spf13/cobra"
)

var (
	errDriftDetected   = errors.New("drift detected")
	errChangesDetected = errors.New("changes detected")
	errMultiScanFailed = errors.New("one or more scans failed")
)

const (
	exitCodeOK            = 0
	exitCodeFailure       = 1
	exitCodeDriftDetected = 2
)

const (
	maxApprovalBytes   = 32 << 20
	maxArtifactBytes   = 32 << 20
	maxManifestBytes   = 32 << 20
	maxDeliveryWorkers = 4
)

func main() {
	if err := newRootCommand(os.Stdout, os.Stderr).Execute(); err != nil {
		code := exitCodeForError(err)
		if code != exitCodeDriftDetected && !errors.Is(err, errMultiScanFailed) {
			fmt.Fprintln(os.Stderr, "Error:", redact.String(err.Error()))
		}
		os.Exit(code)
	}
	os.Exit(exitCodeOK)
}

func exitCodeForError(err error) int {
	if errors.Is(err, errDriftDetected) || errors.Is(err, errChangesDetected) {
		return exitCodeDriftDetected
	}
	return exitCodeFailure
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "terradrift",
		Short:         "Self-hosted Terraform drift detection",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newScanCommand(stdout))
	cmd.AddCommand(newScanAllCommand(stdout))
	cmd.AddCommand(newDashboardIndexCommand(stdout))
	cmd.AddCommand(newServeCommand(stdout))
	cmd.AddCommand(newApproveCommand(stdout))
	cmd.AddCommand(newInitCommand(stdout))
	return cmd
}

func newApproveCommand(stdout io.Writer) *cobra.Command {
	var reportPath string
	var owner string
	var reason string
	var expiresAt string
	var output string
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Create a review-only approval for a drift report",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := ioutil.ReadLimitedFile(reportPath, int64(maxArtifactBytes))
			if err != nil {
				return fmt.Errorf("read report %s: %w", reportPath, err)
			}
			var scanReport report.DriftReport
			if err := json.Unmarshal(data, &scanReport); err != nil {
				return fmt.Errorf("parse report %s: %w", reportPath, err)
			}
			approval, err := report.NewApproval(scanReport, owner, reason, expiresAt)
			if err != nil {
				return err
			}
			if output == "" {
				output = reportPath + ".approval.json"
			}
			output, err = normalizeOutputPath(output)
			if err != nil {
				return err
			}
			data, err = json.MarshalIndent(approval, "", "  ")
			if err != nil {
				return fmt.Errorf("encode approval: %w", err)
			}
			data = append(data, '\n')
			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return fmt.Errorf("create approval %s: %w", output, err)
			}
			if _, err := file.Write(data); err != nil {
				_ = file.Close()
				return fmt.Errorf("write approval %s: %w", output, err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close approval %s: %w", output, err)
			}
			_, err = fmt.Fprintf(stdout, "Created approval: %s\n", output)
			return err
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "JSON drift report to approve")
	cmd.Flags().StringVar(&owner, "owner", "", "approver identity")
	cmd.Flags().StringVar(&reason, "reason", "", "approval reason")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "approval expiry in RFC3339 format")
	cmd.Flags().StringVar(&output, "output", "", "approval artifact path")
	for _, name := range []string{"report", "owner", "reason", "expires-at"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			markErr := err
			cmd.RunE = func(*cobra.Command, []string) error {
				return markErr
			}
			return cmd
		}
	}
	return cmd
}

func newDashboardIndexCommand(stdout io.Writer) *cobra.Command {
	var historyDir string
	var output string
	var limit int
	cmd := &cobra.Command{
		Use:   "dashboard-index",
		Short: "Write a static dashboard index from scan history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				return fmt.Errorf("limit must be greater than zero")
			}
			entries, err := history.LoadRecent(historyDir, limit)
			if err != nil {
				return err
			}
			output, err = normalizeOutputPath(output)
			if err != nil {
				return err
			}
			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("create dashboard index %s: %w", output, err)
			}
			if err := dashboard.RenderIndex(file, entries); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close dashboard index %s: %w", output, err)
			}
			_, err = fmt.Fprintf(stdout, "Wrote dashboard index: %s\n", output)
			return err
		},
	}
	cmd.Flags().StringVar(&historyDir, "history-dir", ".terradrift-history", "directory containing scan history")
	cmd.Flags().StringVar(&output, "output", "terradrift-index.html", "HTML dashboard index path")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum history reports to include")
	return cmd
}

func newServeCommand(stdout io.Writer) *cobra.Command {
	var historyDir string
	var listen string
	var limit int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve local scan history over a read-only API",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateLocalListenAddress(listen); err != nil {
				return err
			}
			if limit <= 0 {
				return fmt.Errorf("limit must be greater than zero")
			}
			listener, err := net.Listen("tcp", listen)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", listen, err)
			}
			server := &http.Server{Handler: newHistoryHandler(historyDir, limit), ReadHeaderTimeout: 5 * time.Second}
			go func() {
				<-cmd.Context().Done()
				_ = server.Shutdown(context.Background())
			}()
			if _, err := fmt.Fprintf(stdout, "Serving scan history at http://%s\n", listener.Addr()); err != nil {
				_ = listener.Close()
				return fmt.Errorf("write server address: %w", err)
			}
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("serve scan history: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&historyDir, "history-dir", ".terradrift-history", "directory containing scan history")
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:8080", "loopback address to listen on")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum reports to serve")
	return cmd
}

func validateLocalListenAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse listen address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("listen address must be loopback-only")
	}
	return nil
}

func newHistoryHandler(historyDir string, limit int) http.Handler {
	load := func() ([]history.Entry, error) {
		return history.LoadRecent(historyDir, limit)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/reports", func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		entries, err := load()
		if err != nil {
			http.Error(w, "load scan history", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(entries); err != nil {
			return
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if request.URL.Path != "/" {
			http.NotFound(w, request)
			return
		}
		entries, err := load()
		if err != nil {
			http.Error(w, "load scan history", http.StatusInternalServerError)
			return
		}
		data := dashboard.Data{History: entries}
		if len(entries) > 0 {
			data.Current = entries[0].Report
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := dashboard.RenderWithHistory(w, data); err != nil {
			return
		}
	})
	return mux
}

func newInitCommand(stdout io.Writer) *cobra.Command {
	var path string
	var directory string
	var terraformExec bool
	var redactPaths bool
	var historyDir string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a tailored TerraDrift config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Default()
			cfg.Directory = directory
			cfg.TerraformExec = terraformExec
			cfg.RedactPaths = redactPaths
			cfg.HistoryDir = historyDir
			if err := config.Write(path, cfg); err != nil {
				return err
			}
			_, err := fmt.Fprintf(stdout, "Created TerraDrift config: %s\n", path)
			return err
		},
	}
	cmd.Flags().StringVar(&path, "config", config.DefaultPath, "config file path to create")
	cmd.Flags().StringVar(&directory, "directory", ".", "Terraform directory for the generated config")
	cmd.Flags().BoolVar(&terraformExec, "terraform-exec", false, "enable Terraform execution in the generated config")
	cmd.Flags().BoolVar(&redactPaths, "redact-paths", false, "redact paths in the generated config")
	cmd.Flags().StringVar(&historyDir, "history-dir", "", "history directory for the generated config")
	return cmd
}

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output path must not be a symlink: %s", path)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect output path %s: %w", path, err)
	}
	return nil
}

func normalizeOutputPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve output path %s: %w", path, err)
	}
	if err := rejectSymlink(absPath); err != nil {
		return "", err
	}
	parent := filepath.Dir(absPath)
	info, err := os.Lstat(parent)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("output parent must not be a symlink: %s", parent)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect output parent %s: %w", parent, err)
	}
	// Canonicalize existing parents so platform symlink roots (macOS /var) do not appear in reports.
	if resolvedParent, err := filepath.EvalSymlinks(parent); err == nil {
		absPath = filepath.Join(resolvedParent, filepath.Base(absPath))
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve output parent %s: %w", parent, err)
	}
	return absPath, nil
}
