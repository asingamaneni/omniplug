// Package claude implements the Adapter for Claude Code. It compiles the
// canonical IR into a self-contained Claude plugin directory:
//
//	.claude-plugin/plugin.json
//	skills/<name>/SKILL.md (+ supporting files)
//	commands/<name>.md
//	agents/<name>.md
//	hooks/hooks.json
//	.mcp.json
//	CLAUDE.md
package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
	"github.com/asingamaneni/omniplug/internal/yamlfm"
)

const name = "claude"

func init() { adapter.Register(&Adapter{}) }

// Adapter is the Claude Code target.
type Adapter struct{}

// Name returns the stable target identifier.
func (a *Adapter) Name() string { return name }

// Capabilities declares Claude's full component support.
func (a *Adapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{
		Skills:   true,
		MCP:      true,
		Commands: adapter.CmdNative,
		Agents:   true,
		Hooks:    true,
		Guidance: true,
	}
}

// modelTier maps an abstract tier to a Claude model alias.
func modelTier(t model.Tier) string {
	switch t {
	case model.TierFast:
		return "haiku"
	case model.TierBalanced:
		return "sonnet"
	case model.TierPowerful:
		return "opus"
	case model.TierInherit:
		return "inherit"
	default:
		return ""
	}
}

// Validate checks Claude-specific constraints.
func (a *Adapter) Validate(p *model.Plugin) []adapter.Diagnostic {
	var ds []adapter.Diagnostic
	for _, h := range p.Hooks {
		if !knownHookEvent(h.Event) {
			ds = append(ds, adapter.Warn(name, "hooks",
				fmt.Sprintf("hook event %q is not a known Claude event (case-sensitive)", h.Event)))
		}
	}
	return ds
}

var claudeHookEvents = map[string]bool{
	"PreToolUse": true, "PostToolUse": true, "UserPromptSubmit": true,
	"Notification": true, "Stop": true, "SubagentStart": true, "SubagentStop": true,
	"PreCompact": true, "SessionStart": true, "SessionEnd": true,
}

func knownHookEvent(e string) bool { return claudeHookEvents[e] }

// Compile transforms the IR into a Claude plugin bundle.
func (a *Adapter) Compile(p *model.Plugin) (adapter.Bundle, []adapter.Diagnostic, error) {
	b := adapter.NewBundle()
	var ds []adapter.Diagnostic

	manifest, err := compileManifest(p)
	if err != nil {
		return b, ds, err
	}
	b.Add(".claude-plugin/plugin.json", manifest)

	for _, s := range p.Skills {
		b.Add(filepath.ToSlash(filepath.Join("skills", s.Name, "SKILL.md")), compileSkill(s))
		for _, f := range s.Files {
			b.AddFile(filepath.ToSlash(filepath.Join("skills", s.Name, f.RelPath)), f.Content, f.Mode)
		}
	}
	for _, c := range p.Commands {
		b.Add(filepath.ToSlash(filepath.Join("commands", c.Name+".md")), compileCommand(c))
	}
	for _, ag := range p.Agents {
		b.Add(filepath.ToSlash(filepath.Join("agents", ag.Name+".md")), compileAgent(ag))
	}
	if len(p.Hooks) > 0 {
		hb, err := compileHooks(p.Hooks)
		if err != nil {
			return b, ds, err
		}
		b.Add("hooks/hooks.json", hb)
	}
	for _, f := range p.HookFiles {
		b.AddFile(filepath.ToSlash(f.RelPath), f.Content, f.Mode)
	}
	if len(p.MCPServers) > 0 {
		mb, err := compileMCP(p.MCPServers)
		if err != nil {
			return b, ds, err
		}
		b.Add(".mcp.json", mb)
	}
	if p.Guidance != nil && p.Guidance.Body != "" {
		b.Add("CLAUDE.md", []byte(p.Guidance.Body))
	}
	return b, ds, nil
}

// InstallPlan resolves the install root for the given scope.
func (a *Adapter) InstallPlan(p *model.Plugin, scope adapter.Scope, projectDir string) (adapter.InstallPlan, error) {
	switch scope {
	case adapter.ScopeProject:
		root := filepath.Join(projectDir, ".claude", "plugins", p.Name)
		return adapter.InstallPlan{Root: root, Description: "project plugin dir (.claude/plugins/" + p.Name + ")"}, nil
	case adapter.ScopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return adapter.InstallPlan{}, err
		}
		root := filepath.Join(home, ".claude", "plugins", p.Name)
		return adapter.InstallPlan{Root: root, Description: "user plugin dir (~/.claude/plugins/" + p.Name + ")"}, nil
	default:
		return adapter.InstallPlan{}, fmt.Errorf("unknown scope %q", scope)
	}
}

// ---- component compilers ----

type jsonManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Author      *struct {
		Name string `json:"name,omitempty"`
		URL  string `json:"url,omitempty"`
	} `json:"author,omitempty"`
	License    string   `json:"license,omitempty"`
	Homepage   string   `json:"homepage,omitempty"`
	Repository string   `json:"repository,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
}

func compileManifest(p *model.Plugin) ([]byte, error) {
	m := jsonManifest{
		Name: p.Name, Version: p.Version, Description: p.Description,
		License: p.License, Homepage: p.Homepage, Repository: p.Repository,
		Keywords: p.Keywords,
	}
	if p.Author.Name != "" || p.Author.URL != "" {
		m.Author = &struct {
			Name string `json:"name,omitempty"`
			URL  string `json:"url,omitempty"`
		}{Name: p.Author.Name, URL: p.Author.URL}
	}
	return marshalJSON(m)
}

func compileSkill(s model.Skill) []byte {
	b := &yamlfm.Builder{}
	b.Scalar("name", s.Name)
	b.Scalar("description", s.Description)
	b.Scalar("when_to_use", s.WhenToUse)
	b.Scalar("argument-hint", s.ArgumentHint)
	b.List("arguments", s.Arguments)
	// autoInvoke: false -> disable-model-invocation: true
	if s.AutoInvoke != nil && !*s.AutoInvoke {
		b.Bool("disable-model-invocation", true)
	}
	if s.UserInvocable != nil && !*s.UserInvocable {
		b.Bool("user-invocable", false)
	}
	b.List("allowed-tools", s.AllowedTools)
	b.List("disallowed-tools", s.DisallowedTools)
	b.Raw("model", modelTier(s.Model))
	b.Raw("effort", s.Effort)
	b.List("paths", s.Globs)
	if s.RunInSubagent {
		b.Raw("context", "fork")
	}
	b.Targets(s.Targets[name])
	return b.Render(s.Body)
}

func compileCommand(c model.Command) []byte {
	b := &yamlfm.Builder{}
	b.Scalar("description", c.Description)
	b.Scalar("argument-hint", c.ArgumentHint)
	b.List("allowed-tools", c.AllowedTools)
	b.Raw("model", modelTier(c.Model))
	// Commands are explicit by definition.
	b.Bool("disable-model-invocation", true)
	b.Targets(c.Targets[name])
	return b.Render(c.Body)
}

func compileAgent(ag model.Agent) []byte {
	b := &yamlfm.Builder{}
	b.Scalar("name", ag.Name)
	b.Scalar("description", ag.Description)
	b.List("tools", ag.Tools)
	b.List("disallowedTools", ag.DisallowedTools)
	b.Raw("model", modelTier(ag.Model))
	if ag.MaxTurns > 0 {
		b.Raw("maxTurns", fmt.Sprintf("%d", ag.MaxTurns))
	}
	b.Scalar("color", ag.Color)
	b.Targets(ag.Targets[name])
	return b.Render(ag.Body)
}

func compileHooks(hooks []model.Hook) ([]byte, error) {
	type hookEntry struct {
		Type    string `json:"type"`
		Command string `json:"command,omitempty"`
	}
	type matcherGroup struct {
		Matcher string      `json:"matcher,omitempty"`
		Hooks   []hookEntry `json:"hooks"`
	}
	byEvent := map[string][]matcherGroup{}
	for _, h := range hooks {
		byEvent[h.Event] = append(byEvent[h.Event], matcherGroup{
			Matcher: h.Matcher,
			Hooks:   []hookEntry{{Type: h.Type, Command: pluginCommand(h.Command)}},
		})
	}
	// Plugin hooks.json wraps the event map under a top-level "hooks" key.
	return marshalJSON(map[string]map[string][]matcherGroup{"hooks": byEvent})
}

// pluginCommand rewrites a plugin-root-relative command (`./hooks/x.sh`) to use
// the ${CLAUDE_PLUGIN_ROOT} placeholder, which is required for bundled scripts:
// a plugin hook does not run with the plugin directory as its working directory.
func pluginCommand(cmd string) string {
	if strings.HasPrefix(cmd, "./") {
		return "${CLAUDE_PLUGIN_ROOT}/" + cmd[len("./"):]
	}
	return cmd
}

func compileMCP(servers []model.MCPServer) ([]byte, error) {
	type serverJSON struct {
		Type    string            `json:"type,omitempty"`
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
	}
	m := map[string]serverJSON{}
	for _, s := range servers {
		sj := serverJSON{}
		switch s.Transport {
		case "http", "sse":
			sj.Type = s.Transport
			sj.URL = s.URL
		default: // stdio
			sj.Type = "stdio"
			sj.Command = pluginCommand(s.Command)
			sj.Args = pluginArgs(s.Args)
			sj.Env = s.Env
		}
		m[s.Name] = sj
	}
	return marshalJSON(map[string]map[string]serverJSON{"mcpServers": m})
}

// pluginArgs applies pluginCommand to each argument so bundled-file references
// (e.g. ./config.json) resolve via ${CLAUDE_PLUGIN_ROOT}.
func pluginArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = pluginCommand(a)
	}
	return out
}

// marshalJSON produces stable, indented JSON with a trailing newline.
// Map keys are sorted by encoding/json, giving deterministic output.
func marshalJSON(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
