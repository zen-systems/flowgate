package gate

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CommandTemplate defines an allowed command template.
type CommandTemplate struct {
	Exec string
	Args []string
}

var capabilityTemplates = map[string][]CommandTemplate{
	"go_test": {
		{Exec: "go", Args: []string{"test", "./..."}},
		{Exec: "go", Args: []string{"test", "./pkg/..."}},
		{Exec: "go", Args: []string{"test", "./cmd/..."}},
	},
	"go_vet": {
		{Exec: "go", Args: []string{"vet", "./..."}},
	},
	"gofmt": {
		{Exec: "gofmt", Args: []string{"-w", "{path}"}},
	},
}

var allowedPkgArgs = map[string]struct{}{
	"./...":     {},
	"./pkg/...": {},
	"./cmd/...": {},
}

func templatesForCapability(name string) ([]CommandTemplate, bool) {
	templates, ok := capabilityTemplates[name]
	if !ok {
		return nil, false
	}
	return templates, true
}

// TemplatesForCapability returns templates for a capability name.
func TemplatesForCapability(name string) ([]CommandTemplate, bool) {
	return templatesForCapability(name)
}

func matchTemplates(command []string, templates []CommandTemplate, workspaceRoot, workdir string) (bool, string) {
	if len(templates) == 0 {
		return true, ""
	}
	var lastReason string
	for _, tmpl := range templates {
		if tmpl.Exec == "" {
			continue
		}
		if len(command) == 0 || command[0] != tmpl.Exec {
			continue
		}
		if len(command)-1 != len(tmpl.Args) {
			continue
		}
		matched := true
		for i, arg := range tmpl.Args {
			value := command[i+1]
			switch arg {
			case "{path}":
				ok, reason := isWorkspaceConfined(workdir, workspaceRoot, value)
				if !ok {
					matched = false
					lastReason = reason
					break
				}
			case "{pkg}":
				if _, ok := allowedPkgArgs[value]; !ok {
					matched = false
					lastReason = "package argument not allowed"
					break
				}
			default:
				if value != arg {
					matched = false
					break
				}
			}
		}
		if matched {
			return true, ""
		}
	}
	if lastReason != "" {
		return false, lastReason
	}
	return false, "command does not match any allowed template"
}

// isWorkspaceConfined validates that arg stays within workspace.
func isWorkspaceConfined(workdir, workspace, arg string) (bool, string) {
	if workspace == "" {
		return false, "workspace root not set"
	}
	if filepath.IsAbs(arg) {
		return false, "absolute paths are not allowed"
	}
	cleanArg := filepath.Clean(arg)
	if cleanArg == "." {
		return false, "invalid path"
	}
	for _, seg := range strings.Split(cleanArg, string(filepath.Separator)) {
		if seg == ".." {
			return false, "path traversal detected"
		}
	}

	base := workspace
	if workdir != "" {
		baseCandidate := workdir
		if !filepath.IsAbs(baseCandidate) {
			baseCandidate = filepath.Join(workspace, baseCandidate)
		}
		if ok, reason := confinedUnderWorkspace(workspace, baseCandidate); !ok {
			return false, fmt.Sprintf("workdir not confined: %s", reason)
		}
		base = baseCandidate
	}

	candidate := filepath.Clean(filepath.Join(base, cleanArg))
	if ok, reason := confinedUnderWorkspace(workspace, candidate); !ok {
		return false, fmt.Sprintf("path not confined: %s", reason)
	}
	return true, ""
}

func confinedUnderWorkspace(workspace, candidate string) (bool, string) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return false, "invalid workspace"
	}
	cand, err := filepath.Abs(candidate)
	if err != nil {
		return false, "invalid path"
	}
	if cand == root {
		return true, ""
	}
	if strings.HasPrefix(cand, root+string(filepath.Separator)) {
		return true, ""
	}
	return false, "path escapes workspace"
}
