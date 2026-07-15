// Package schema validates a parsed plugin against the canonical rules.
// Errors block compilation; warnings (e.g. unknown/degraded fields) do not.
package schema

import (
	"fmt"
	"regexp"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
)

const source = "schema"

// nameRe restricts names that become path segments to a safe character set,
// preventing path-traversal via crafted skill/command/agent/plugin names.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var validHookTypes = map[string]bool{
	"command": true, "http": true, "mcp_tool": true, "prompt": true, "agent": true,
}
var validTransports = map[string]bool{"stdio": true, "http": true, "sse": true}

// Validate checks structural rules: required fields, safe names, enum membership,
// and uniqueness of names within each component type.
func Validate(p *model.Plugin) []adapter.Diagnostic {
	var ds []adapter.Diagnostic

	if p.APIVersion != "" && p.APIVersion != model.APIVersion {
		ds = append(ds, adapter.Warn(source, "manifest",
			fmt.Sprintf("apiVersion %q does not match supported %q", p.APIVersion, model.APIVersion)))
	}
	if p.Name == "" {
		ds = append(ds, adapter.Error(source, "manifest", "plugin name is required"))
	} else {
		ds = append(ds, checkName("manifest", p.Name)...)
	}

	skillNames := map[string]bool{}
	for _, s := range p.Skills {
		comp := "skill:" + s.Name
		ds = append(ds, requireNameDesc(comp, s.Name, s.Description)...)
		ds = append(ds, checkName(comp, s.Name)...)
		ds = append(ds, checkDup(comp, s.Name, skillNames)...)
		ds = append(ds, checkModel(comp, s.Model)...)
		ds = append(ds, checkEffort(comp, s.Effort)...)
	}
	cmdNames := map[string]bool{}
	for _, c := range p.Commands {
		comp := "command:" + c.Name
		ds = append(ds, requireNameDesc(comp, c.Name, c.Description)...)
		ds = append(ds, checkName(comp, c.Name)...)
		ds = append(ds, checkDup(comp, c.Name, cmdNames)...)
		ds = append(ds, checkModel(comp, c.Model)...)
	}
	agentNames := map[string]bool{}
	for _, a := range p.Agents {
		comp := "agent:" + a.Name
		ds = append(ds, requireNameDesc(comp, a.Name, a.Description)...)
		ds = append(ds, checkName(comp, a.Name)...)
		ds = append(ds, checkDup(comp, a.Name, agentNames)...)
		ds = append(ds, checkModel(comp, a.Model)...)
	}
	for _, h := range p.Hooks {
		if h.Event == "" {
			ds = append(ds, adapter.Error(source, "hooks", "hook is missing required 'event'"))
		}
		if h.Type != "" && !validHookTypes[h.Type] {
			ds = append(ds, adapter.Error(source, "hooks",
				fmt.Sprintf("invalid hook type %q (want command, http, mcp_tool, prompt, or agent)", h.Type)))
		}
	}
	mcpNames := map[string]bool{}
	for _, m := range p.MCPServers {
		comp := "mcp:" + m.Name
		if m.Name == "" {
			ds = append(ds, adapter.Error(source, "mcp", "MCP server is missing required 'name'"))
		} else {
			// MCP names become JSON map keys; duplicates would silently drop a server.
			ds = append(ds, checkDup(comp, m.Name, mcpNames)...)
		}
		if m.Transport != "" && !validTransports[m.Transport] {
			ds = append(ds, adapter.Error(source, comp,
				fmt.Sprintf("invalid transport %q (want stdio, http, or sse)", m.Transport)))
		}
	}
	return ds
}

func requireNameDesc(comp, name, desc string) []adapter.Diagnostic {
	var ds []adapter.Diagnostic
	if name == "" {
		ds = append(ds, adapter.Error(source, comp, "missing required 'name'"))
	}
	if desc == "" {
		ds = append(ds, adapter.Error(source, comp, "missing required 'description'"))
	}
	return ds
}

func checkName(comp, name string) []adapter.Diagnostic {
	if name == "" || nameRe.MatchString(name) {
		return nil
	}
	return []adapter.Diagnostic{adapter.Error(source, comp,
		fmt.Sprintf("invalid name %q (allowed: letters, digits, '.', '_', '-'; no path separators or '..')", name))}
}

func checkDup(comp, name string, seen map[string]bool) []adapter.Diagnostic {
	if name == "" {
		return nil
	}
	if seen[name] {
		return []adapter.Diagnostic{adapter.Error(source, comp,
			fmt.Sprintf("duplicate name %q within its component type", name))}
	}
	seen[name] = true
	return nil
}

func checkModel(comp string, m model.ModelTier) []adapter.Diagnostic {
	if m == model.TierUnset {
		return nil
	}
	for _, valid := range model.ValidModelTiers {
		if m == valid {
			return nil
		}
	}
	return []adapter.Diagnostic{adapter.Error(source, comp,
		fmt.Sprintf("invalid model tier %q (want one of fast, balanced, powerful, inherit)", m))}
}

func checkEffort(comp, e string) []adapter.Diagnostic {
	if e == "" {
		return nil
	}
	for _, valid := range model.ValidEfforts {
		if e == valid {
			return nil
		}
	}
	return []adapter.Diagnostic{adapter.Error(source, comp,
		fmt.Sprintf("invalid effort %q (want one of low, medium, high)", e))}
}
