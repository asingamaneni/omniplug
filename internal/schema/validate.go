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

// validHookTypes is intentionally command-only: the IR has no payload fields
// for other hook kinds yet, so accepting them would emit broken output.
var validHookTypes = map[string]bool{"command": true}

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
		ds = append(ds, checkHook(h)...)
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
		ds = append(ds, checkMCPServer(comp, m)...)
	}
	return ds
}

// checkHook enforces that a hook can actually be emitted: a known event, a
// supported type, and a non-empty command (adapters serialize Command verbatim,
// so an empty one would produce a hook entry that does nothing).
func checkHook(h model.Hook) []adapter.Diagnostic {
	var ds []adapter.Diagnostic
	if h.Event == "" {
		ds = append(ds, adapter.Error(source, "hooks", "hook is missing required 'event'"))
	}
	if h.Type != "" && !validHookTypes[h.Type] {
		ds = append(ds, adapter.Error(source, "hooks",
			fmt.Sprintf("invalid hook type %q (only \"command\" is supported in %s)", h.Type, model.APIVersion)))
	} else if h.Command == "" {
		ds = append(ds, adapter.Error(source, "hooks",
			fmt.Sprintf("command hook for event %q is missing required 'command'", h.Event)))
	}
	return ds
}

// checkMCPServer enforces per-transport required fields so adapters never
// serialize a server that its host cannot load (stdio without a command,
// http/sse without a url), and warns about fields the transport ignores.
func checkMCPServer(comp string, m model.MCPServer) []adapter.Diagnostic {
	var ds []adapter.Diagnostic
	switch m.Transport {
	case "", "stdio":
		if m.Command == "" {
			ds = append(ds, adapter.Error(source, comp, "stdio MCP server requires 'command'"))
		}
		if m.URL != "" {
			ds = append(ds, adapter.Warn(source, comp,
				"'url' is ignored for stdio transport (did you mean transport: http or sse?)"))
		}
	case "http", "sse":
		if m.URL == "" {
			ds = append(ds, adapter.Error(source, comp,
				fmt.Sprintf("%s MCP server requires 'url'", m.Transport)))
		}
		// Remote servers carry only a url downstream; command/args/env are not
		// expressible, so surface the drop rather than losing (e.g.) an auth
		// token silently. Authenticate remote servers via the url or headers.
		for _, f := range ignoredRemoteFields(m) {
			ds = append(ds, adapter.Warn(source, comp,
				fmt.Sprintf("'%s' is ignored for %s transport (remote servers take only 'url')", f, m.Transport)))
		}
	default:
		ds = append(ds, adapter.Error(source, comp,
			fmt.Sprintf("invalid transport %q (want stdio, http, or sse)", m.Transport)))
	}
	return ds
}

// ignoredRemoteFields lists the stdio-only fields set on a remote (http/sse)
// server, in a stable order, which downstream MCP configs cannot express.
func ignoredRemoteFields(m model.MCPServer) []string {
	var out []string
	if m.Command != "" {
		out = append(out, "command")
	}
	if len(m.Args) > 0 {
		out = append(out, "args")
	}
	if len(m.Env) > 0 {
		out = append(out, "env")
	}
	return out
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

func checkModel(comp string, m model.Tier) []adapter.Diagnostic {
	if m == model.TierUnset {
		return nil
	}
	for _, valid := range model.ValidTiers {
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
		fmt.Sprintf("invalid effort %q (want one of low, medium, high, xhigh, max)", e))}
}
