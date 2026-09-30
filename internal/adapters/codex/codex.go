// Package codex implements the Adapter for the OpenAI Codex CLI. It compiles
// the canonical IR into a Codex plugin directory, the unit `codex plugin add`
// installs from a marketplace:
//
//	.codex-plugin/plugin.json
//	skills/<name>/SKILL.md (+ supporting files)
//	hooks/hooks.json
//	.mcp.json
//
// Codex plugins carry skills, MCP servers and hooks. They have no slash
// commands, no subagents and no guidance file, so those degrade with a
// diagnostic: commands become skills, agents and guidance are dropped.
package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
	"github.com/asingamaneni/omniplug/internal/yamlfm"
)

const name = "codex"

func init() { adapter.Register(&Adapter{}) }

// Adapter is the Codex CLI target.
type Adapter struct{}

// Name returns the stable target identifier.
func (a *Adapter) Name() string { return name }

// Capabilities declares what a Codex plugin can express.
func (a *Adapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{
		Skills:   true,
		MCP:      true,
		Commands: adapter.CmdNone,
		Agents:   false,
		Hooks:    true,
		Guidance: false,
	}
}

// codexHookEvents are the hook events Codex dispatches.
var codexHookEvents = map[string]bool{
	"PreToolUse": true, "PermissionRequest": true, "PostToolUse": true,
	"PreCompact": true, "PostCompact": true, "SessionStart": true, "SessionEnd": true,
	"UserPromptSubmit": true, "SubagentStart": true, "SubagentStop": true,
	"Stop": true, "Interrupt": true,
}

// Validate checks Codex-specific constraints. Codex shows the author as the
// plugin card's developer, so an empty one leaves the card without it.
func (a *Adapter) Validate(p *model.Plugin) []adapter.Diagnostic {
	var ds []adapter.Diagnostic
	if p.Author.Name == "" {
		ds = append(ds, adapter.Warn(name, "manifest",
			"author.name is empty; Codex shows it as the plugin's developer, set it or override targets.codex.interface.developerName"))
	}
	return ds
}

// Compile transforms the IR into a Codex plugin bundle.
func (a *Adapter) Compile(p *model.Plugin) (adapter.Bundle, []adapter.Diagnostic, error) {
	b := adapter.NewBundle()
	var ds []adapter.Diagnostic

	for _, s := range p.Skills {
		skill, sd := compileSkill(s)
		b.Add(rel("skills", s.Name, "SKILL.md"), skill)
		ds = append(ds, sd...)
		ds = append(ds, claudeRootWarning("skill:"+s.Name, s.Body)...)
		for _, f := range s.Files {
			b.AddFile(rel("skills", s.Name, f.RelPath), f.Content, f.Mode)
		}
	}
	for _, c := range p.Commands {
		b.Add(rel("skills", c.Name, "SKILL.md"), compileCommandSkill(c))
		ds = append(ds, claudeRootWarning("command:"+c.Name, c.Body)...)
		ds = append(ds, adapter.Warn(name, "command:"+c.Name,
			"Codex plugins have no slash commands; emitted as a skill the model may invoke"))
		if len(c.AllowedTools) > 0 || c.Model != model.TierUnset || c.ArgumentHint != "" {
			ds = append(ds, adapter.Warn(name, "command:"+c.Name,
				"Codex skills cannot express allowedTools, model or argumentHint; dropped"))
		}
	}
	for _, ag := range p.Agents {
		ds = append(ds, adapter.Warn(name, "agent:"+ag.Name,
			"Codex plugins cannot carry subagents; dropped"))
	}

	if len(p.Hooks) > 0 {
		hb, hd, err := compileHooks(p.Hooks)
		if err != nil {
			return b, ds, err
		}
		ds = append(ds, hd...)
		if hb != nil {
			b.Add("hooks/hooks.json", hb)
			ds = append(ds, adapter.Warn(name, "hooks",
				"Codex runs a plugin's hooks only after the user trusts them; until then they are skipped, and codex exec skips them without a message"))
		}
	}
	// Bundled scripts ship even when no hook survives: an MCP server may use them.
	for _, f := range p.HookFiles {
		b.AddFile(filepath.ToSlash(f.RelPath), f.Content, f.Mode)
	}

	servers, md := compileMCP(p.MCPServers)
	ds = append(ds, md...)
	if servers != nil {
		mb, err := marshalJSON(map[string]any{"mcpServers": servers})
		if err != nil {
			return b, ds, err
		}
		b.Add(".mcp.json", mb)
	}

	if p.Guidance != nil && p.Guidance.Body != "" {
		ds = append(ds, adapter.Warn(name, "guidance",
			"Codex plugins cannot carry guidance; put it in the project's AGENTS.md"))
	}

	manifest, err := compileManifest(p, len(p.Skills)+len(p.Commands) > 0, servers != nil)
	if err != nil {
		return b, ds, err
	}
	b.Add(".codex-plugin/plugin.json", manifest)
	return b, ds, nil
}

// InstallPlan resolves where the plugin directory goes. Codex installs plugins
// from a marketplace, whose entries point at ./plugins/<name> relative to the
// marketplace root: the home directory for the personal marketplace
// (~/.agents/plugins/marketplace.json), the repository for a project one.
func (a *Adapter) InstallPlan(p *model.Plugin, scope adapter.Scope, projectDir string) (adapter.InstallPlan, error) {
	switch scope {
	case adapter.ScopeProject:
		root := filepath.Join(projectDir, "plugins", p.Name)
		return adapter.InstallPlan{Root: root, Description: "project plugin dir (plugins/" + p.Name +
			"); not registered yet: add it to .agents/plugins/marketplace.json, then run codex plugin add"}, nil
	case adapter.ScopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return adapter.InstallPlan{}, err
		}
		root := filepath.Join(home, "plugins", p.Name)
		return adapter.InstallPlan{Root: root, Description: "personal plugin dir (~/plugins/" + p.Name +
			"); not registered yet: add it to ~/.agents/plugins/marketplace.json, then run codex plugin add"}, nil
	default:
		return adapter.InstallPlan{}, fmt.Errorf("unknown scope %q", scope)
	}
}

// ---- component compilers ----

// compileManifest builds .codex-plugin/plugin.json. Codex draws the plugin card
// from the interface block, so the card is derived from the canonical metadata;
// the targets.codex escape hatch can override any field, including interface.
func compileManifest(p *model.Plugin, hasSkills, hasMCP bool) ([]byte, error) {
	m := map[string]any{"name": p.Name, "version": p.Version}
	setNonEmpty(m, "description", p.Description)
	setNonEmpty(m, "license", p.License)
	setNonEmpty(m, "homepage", p.Homepage)
	setNonEmpty(m, "repository", p.Repository)
	if len(p.Keywords) > 0 {
		m["keywords"] = p.Keywords
	}
	if p.Author.Name != "" || p.Author.URL != "" {
		a := map[string]any{}
		setNonEmpty(a, "name", p.Author.Name)
		setNonEmpty(a, "url", p.Author.URL)
		m["author"] = a
	}
	if hasSkills {
		m["skills"] = "./skills/"
	}
	if hasMCP {
		m["mcpServers"] = "./.mcp.json"
	}
	m["interface"] = map[string]any{
		"displayName":      p.Name,
		"shortDescription": p.Description,
		"longDescription":  p.Description,
		"developerName":    p.Author.Name,
		"category":         "Productivity",
		"capabilities":     []string{"Interactive"},
		"defaultPrompt":    []string{truncate(p.Description, 128)},
	}
	for k, v := range p.Targets[name] {
		if k == "interface" {
			if over, ok := v.(map[string]any); ok {
				merged := m["interface"].(map[string]any)
				for ik, iv := range over {
					merged[ik] = iv
				}
				continue
			}
		}
		m[k] = v
	}
	return marshalJSON(m)
}

// compileSkill writes a SKILL.md with the fields Codex reads (name,
// description). Everything Claude-specific is dropped with a diagnostic.
func compileSkill(s model.Skill) ([]byte, []adapter.Diagnostic) {
	var ds []adapter.Diagnostic
	b := &yamlfm.Builder{}
	b.Scalar("name", s.Name)
	b.Scalar("description", description(s.Description, s.WhenToUse))
	b.Targets(s.Targets[name])

	var dropped []string
	if len(s.AllowedTools) > 0 || len(s.DisallowedTools) > 0 {
		dropped = append(dropped, "tool restrictions")
	}
	if s.Model != model.TierUnset {
		dropped = append(dropped, "model")
	}
	if s.Effort != "" {
		dropped = append(dropped, "effort")
	}
	if s.AutoInvoke != nil && !*s.AutoInvoke {
		dropped = append(dropped, "autoInvoke: false")
	}
	if s.UserInvocable != nil && !*s.UserInvocable {
		dropped = append(dropped, "userInvocable: false")
	}
	if len(s.Globs) > 0 {
		dropped = append(dropped, "globs")
	}
	if s.RunInSubagent {
		dropped = append(dropped, "runInSubagent")
	}
	if s.ArgumentHint != "" || len(s.Arguments) > 0 {
		dropped = append(dropped, "arguments")
	}
	if len(dropped) > 0 {
		ds = append(ds, adapter.Warn(name, "skill:"+s.Name,
			"Codex skills cannot express "+strings.Join(dropped, ", ")+"; dropped"))
	}
	return b.Render(s.Body), ds
}

// compileCommandSkill represents a canonical command as a Codex skill.
func compileCommandSkill(c model.Command) []byte {
	b := &yamlfm.Builder{}
	b.Scalar("name", c.Name)
	b.Scalar("description", c.Description)
	b.Targets(c.Targets[name])
	return b.Render(c.Body)
}

// claudeRootWarning reports a body that names $CLAUDE_PLUGIN_ROOT. Codex does
// not substitute it in skill text and its shell does not carry it, so a command
// written as "${CLAUDE_PLUGIN_ROOT}/scripts/x" runs against "/scripts/x".
func claudeRootWarning(component, body string) []adapter.Diagnostic {
	if !strings.Contains(body, "$CLAUDE_PLUGIN_ROOT") && !strings.Contains(body, "${CLAUDE_PLUGIN_ROOT}") {
		return nil
	}
	return []adapter.Diagnostic{adapter.Warn(name, component,
		"names $CLAUDE_PLUGIN_ROOT, which Codex does not fill in in skill text; the path resolves against / unless the session is told the plugin's directory")}
}

// description folds when-to-use into the description, the only trigger text a
// Codex skill has.
func description(desc, whenToUse string) string {
	if whenToUse == "" {
		return desc
	}
	if desc == "" {
		return whenToUse
	}
	return desc + " " + whenToUse
}

// compileHooks writes hooks.json in the matcher-group shape Codex reads. Events
// Codex does not dispatch are dropped with a diagnostic; when none remain the
// file is not emitted.
func compileHooks(hooks []model.Hook) ([]byte, []adapter.Diagnostic, error) {
	type hookEntry struct {
		Type    string `json:"type"`
		Command string `json:"command,omitempty"`
	}
	type matcherGroup struct {
		Matcher string      `json:"matcher,omitempty"`
		Hooks   []hookEntry `json:"hooks"`
	}
	var ds []adapter.Diagnostic
	byEvent := map[string][]matcherGroup{}
	for _, h := range hooks {
		if !codexHookEvents[h.Event] {
			ds = append(ds, adapter.Warn(name, "hooks",
				fmt.Sprintf("hook event %q is not dispatched by Codex; dropped", h.Event)))
			continue
		}
		byEvent[h.Event] = append(byEvent[h.Event], matcherGroup{
			Matcher: h.Matcher,
			Hooks:   []hookEntry{{Type: h.Type, Command: pluginPath(h.Command)}},
		})
	}
	if len(byEvent) == 0 {
		return nil, ds, nil
	}
	out, err := marshalJSON(map[string]map[string][]matcherGroup{"hooks": byEvent})
	return out, ds, err
}

// pluginPath rewrites a plugin-root-relative hook command (`./hooks/x.sh`) to
// ${PLUGIN_ROOT}, which Codex sets in a plugin hook's environment: a hook does
// not run with the plugin directory as its working directory.
func pluginPath(s string) string {
	if strings.HasPrefix(s, "./") {
		return "${PLUGIN_ROOT}/" + s[len("./"):]
	}
	return s
}

// compileMCP maps servers to Codex's .mcp.json shape. Codex does not support
// the legacy SSE transport, so SSE servers are dropped with a diagnostic.
func compileMCP(servers []model.MCPServer) (map[string]any, []adapter.Diagnostic) {
	if len(servers) == 0 {
		return nil, nil
	}
	var ds []adapter.Diagnostic
	out := map[string]any{}
	names := make([]string, 0, len(servers))
	for _, s := range servers {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	byName := map[string]model.MCPServer{}
	for _, s := range servers {
		byName[s.Name] = s
	}
	for _, n := range names {
		s := byName[n]
		switch s.Transport {
		case "sse":
			ds = append(ds, adapter.Warn(name, "mcp:"+s.Name,
				"Codex does not support the legacy SSE transport; dropped"))
		case "http":
			out[s.Name] = map[string]any{"type": "http", "url": s.URL}
		default:
			// Codex does not expand ${PLUGIN_ROOT} in .mcp.json; it resolves a
			// server's relative cwd against the plugin directory. So a server that
			// names a bundled file keeps the relative path and runs from there.
			sv := map[string]any{"type": "stdio", "command": s.Command}
			bundled := strings.HasPrefix(s.Command, "./")
			if len(s.Args) > 0 {
				sv["args"] = s.Args
				for _, a := range s.Args {
					bundled = bundled || strings.HasPrefix(a, "./")
				}
			}
			if bundled {
				sv["cwd"] = "."
			}
			if env, forward, bad := codexEnv(s.Env); len(env)+len(forward) > 0 {
				if len(env) > 0 {
					sv["env"] = env
				}
				if len(forward) > 0 {
					sv["env_vars"] = forward
				}
				for _, k := range bad {
					ds = append(ds, adapter.Warn(name, "mcp:"+s.Name,
						fmt.Sprintf("env %s names a variable, which Codex passes as literal text; only NAME: ${NAME} can be forwarded (env_vars)", k)))
				}
			}
			out[s.Name] = sv
		}
	}
	if len(out) == 0 {
		return nil, ds
	}
	return out, ds
}

// codexEnv splits a server's env for Codex, which passes env values literally.
// A value that is exactly ${NAME} for its own NAME is forwarded from the user's
// environment through env_vars; any other value that names a variable is kept
// as written and reported, because Codex cannot express it.
func codexEnv(in map[string]string) (env map[string]string, forward, bad []string) {
	env = map[string]string{}
	for k, v := range in {
		switch {
		case v == "${"+k+"}":
			forward = append(forward, k)
		case strings.Contains(v, "${"):
			env[k] = v
			bad = append(bad, k)
		default:
			env[k] = v
		}
	}
	sort.Strings(forward)
	sort.Strings(bad)
	return env, forward, bad
}

func rel(parts ...string) string { return filepath.ToSlash(filepath.Join(parts...)) }

func setNonEmpty(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// marshalJSON produces stable, indented JSON with a trailing newline.
func marshalJSON(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
