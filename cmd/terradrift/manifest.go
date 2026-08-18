package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/niravraychura/terradrift/internal/config"
	"github.com/niravraychura/terradrift/internal/ioutil"
	"github.com/niravraychura/terradrift/internal/scanner"
	"github.com/niravraychura/terradrift/internal/terraform"
)

// manifestRoot describes one Terraform root, optionally with per-root overrides.
type manifestRoot struct {
	Directory string   `json:"directory"`
	Profile   string   `json:"profile,omitempty"`
	PlanMode  string   `json:"plan_mode,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	VarFiles  []string `json:"var_files,omitempty"`
	Vars      []string `json:"vars,omitempty"`
}

type jsonManifest struct {
	Version int            `json:"version"`
	Roots   []manifestRoot `json:"roots"`
}

// rootDefaults are CLI-wide defaults applied before per-root / profile overlays.
type rootDefaults struct {
	PlanMode  string
	Workspace string
	VarFiles  []string
	Vars      []string
	Config    string
}

func loadScanManifest(path string) ([]manifestRoot, error) {
	data, err := ioutil.ReadLimitedFile(path, int64(maxManifestBytes))
	if err != nil {
		return nil, fmt.Errorf("read scan manifest %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("scan manifest %s has no Terraform roots", path)
	}
	base := filepath.Dir(path)
	if trimmed[0] == '{' {
		return loadJSONManifest(path, base, data)
	}
	return loadTextManifest(path, base, data)
}

func loadJSONManifest(path, base string, data []byte) ([]manifestRoot, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document jsonManifest
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse scan manifest %s: %w", path, err)
	}
	if document.Version != 1 {
		return nil, fmt.Errorf("unsupported scan manifest version %d in %s", document.Version, path)
	}
	if len(document.Roots) == 0 {
		return nil, fmt.Errorf("scan manifest %s has no Terraform roots", path)
	}
	roots := make([]manifestRoot, 0, len(document.Roots))
	for index, root := range document.Roots {
		directory := strings.TrimSpace(root.Directory)
		if directory == "" {
			return nil, fmt.Errorf("scan manifest %s roots[%d] is missing directory", path, index)
		}
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(base, directory)
		}
		root.Directory = directory
		root.Profile = strings.TrimSpace(root.Profile)
		root.PlanMode = strings.TrimSpace(root.PlanMode)
		root.Workspace = strings.TrimSpace(root.Workspace)
		if root.PlanMode != "" {
			if _, err := terraform.ParsePlanMode(root.PlanMode); err != nil {
				return nil, fmt.Errorf("scan manifest %s roots[%d]: %w", path, index, err)
			}
		}
		roots = append(roots, root)
	}
	return roots, nil
}

func loadTextManifest(path, base string, data []byte) ([]manifestRoot, error) {
	roots := make([]manifestRoot, 0)
	for _, line := range strings.Split(string(data), "\n") {
		directory := strings.TrimSpace(line)
		if directory == "" || strings.HasPrefix(directory, "#") {
			continue
		}
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(base, directory)
		}
		roots = append(roots, manifestRoot{Directory: directory})
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("scan manifest %s has no Terraform roots", path)
	}
	return roots, nil
}

func directoriesFromRoots(roots []manifestRoot) []string {
	directories := make([]string, len(roots))
	for index, root := range roots {
		directories[index] = root.Directory
	}
	return directories
}

func filterRoots(roots []manifestRoot, directories []string) []manifestRoot {
	allowed := make(map[string]struct{}, len(directories))
	for _, directory := range directories {
		allowed[directory] = struct{}{}
	}
	filtered := make([]manifestRoot, 0, len(directories))
	for _, root := range roots {
		if _, ok := allowed[root.Directory]; ok {
			filtered = append(filtered, root)
		}
	}
	return filtered
}

func resolveRootOptions(root manifestRoot, defaults rootDefaults, base scanner.Options) (scanner.Options, error) {
	planMode := defaults.PlanMode
	workspace := defaults.Workspace
	varFiles := append([]string(nil), defaults.VarFiles...)
	vars := append([]string(nil), defaults.Vars...)

	if root.Profile != "" {
		if defaults.Config == "" {
			return scanner.Options{}, fmt.Errorf("root %s uses profile %q but --config was not set", root.Directory, root.Profile)
		}
		cfg, err := config.LoadProfile(defaults.Config, root.Profile)
		if err != nil {
			return scanner.Options{}, err
		}
		if cfg.PlanMode != "" {
			planMode = cfg.PlanMode
		}
		if cfg.Workspace != "" {
			workspace = cfg.Workspace
		}
		if len(cfg.VarFiles) > 0 {
			varFiles = append([]string(nil), cfg.VarFiles...)
		}
		if len(cfg.Vars) > 0 {
			vars = append([]string(nil), cfg.Vars...)
		}
		if cfg.TerraformBin != "" {
			if runner, ok := base.Runner.(terraform.CLIRunner); ok {
				runner.Path = cfg.TerraformBin
				base.Runner = runner
			}
		}
	}
	if root.PlanMode != "" {
		planMode = root.PlanMode
	}
	if root.Workspace != "" {
		workspace = root.Workspace
	}
	if len(root.VarFiles) > 0 {
		varFiles = append([]string(nil), root.VarFiles...)
	}
	if len(root.Vars) > 0 {
		vars = append([]string(nil), root.Vars...)
	}

	mode, err := terraform.ParsePlanMode(planMode)
	if err != nil {
		return scanner.Options{}, err
	}
	options := base
	options.Directory = root.Directory
	options.PlanMode = mode
	options.Runner = cloneRunnerWithOverrides(base.Runner, workspace, varFiles, vars)
	return options, nil
}

func cloneRunnerWithOverrides(base terraform.Runner, workspace string, varFiles, vars []string) terraform.Runner {
	if base == nil {
		return nil
	}
	switch runner := base.(type) {
	case terraform.CLIRunner:
		runner.Workspace = workspace
		runner.VarFiles = append([]string(nil), varFiles...)
		runner.Vars = append([]string(nil), vars...)
		return runner
	case *terraform.CLIRunner:
		copy := *runner
		copy.Workspace = workspace
		copy.VarFiles = append([]string(nil), varFiles...)
		copy.Vars = append([]string(nil), vars...)
		return &copy
	default:
		return base
	}
}
