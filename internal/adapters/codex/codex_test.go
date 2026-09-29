package codex

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
)

func boolPtr(b bool) *bool { return &b }

func samplePlugin() *model.Plugin {
	return &model.Plugin{
		Name: "demo", Version: "1.2.3", Description: "demo plugin",
		Author:  model.Author{Name: "Ashok", URL: "https://example.com/ashok"},
		License: "MIT", Homepage: "https://example.com",
		Repository: "https://github.com/x/demo", Keywords: []string{"ai", "demo"},
		Skills: []model.Skill{{
			Name: "deploy", Description: "Deploy it", WhenToUse: "When shipping.", Model: model.TierBalanced,
			AutoInvoke: boolPtr(false), AllowedTools: []string{"Read"},
			Body:  "Do the deploy.",
			Files: []model.File{{RelPath: "scripts/go.sh", Content: []byte("echo hi\n"), Mode: 0o755}},
		}},
		Commands: []model.Command{{Name: "review", Description: "Review", Body: "Review it."}},
		Agents:   []model.Agent{{Name: "rev", Description: "Reviewer", Body: "You review."}},
		Hooks: []model.Hook{
			{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "./hooks/guard.sh"},
			{Event: "Notification", Type: "command", Command: "./hooks/guard.sh"},
		},
		HookFiles: []model.File{{RelPath: "hooks/guard.sh", Content: []byte("exit 0\n"), Mode: 0o755}},
		MCPServers: []model.MCPServer{
			{Name: "local", Transport: "stdio", Command: "./bin/server", Args: []string{"--config", "./cfg.json"}},
			{Name: "remote", Transport: "http", URL: "https://mcp.example.com/mcp"},
			{Name: "old", Transport: "sse", URL: "https://mcp.example.com/sse"},
		},
		Guidance: &model.Guidance{Body: "Be careful."},
	}
}

func compile(t *testing.T, p *model.Plugin) (adapter.Bundle, []adapter.Diagnostic) {
	t.Helper()
	b, ds, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return b, ds
}

func hasWarning(ds []adapter.Diagnostic, component, fragment string) bool {
	for _, d := range ds {
		if d.Severity == adapter.SeverityWarning && d.Component == component && strings.Contains(d.Message, fragment) {
			return true
		}
	}
	return false
}

func TestCompileProducesCodexPluginLayout(t *testing.T) {
	b, _ := compile(t, samplePlugin())
	want := []string{
		".codex-plugin/plugin.json",
		"skills/deploy/SKILL.md",
		"skills/deploy/scripts/go.sh",
		"skills/review/SKILL.md",
		"hooks/hooks.json",
		"hooks/guard.sh",
		".mcp.json",
	}
	for _, w := range want {
		if _, ok := b.Files[w]; !ok {
			t.Errorf("missing %s", w)
		}
	}
	if len(b.Files) != len(want) {
		t.Errorf("got %d files, want %d: %v", len(b.Files), len(want), keys(b.Files))
	}
	if b.Modes["hooks/guard.sh"] != 0o755 || b.Modes["skills/deploy/scripts/go.sh"] != 0o755 {
		t.Errorf("bundled scripts lost their exec bit: %v", b.Modes)
	}
}

// The manifest must pass Codex's own plugin validation: only its accepted
// top-level fields, and an interface block with every required field.
func TestManifestUsesOnlyCodexFieldsAndCarriesInterface(t *testing.T) {
	b, _ := compile(t, samplePlugin())
	var m map[string]any
	if err := json.Unmarshal(b.Files[".codex-plugin/plugin.json"], &m); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"id": true, "name": true, "version": true, "description": true, "skills": true,
		"apps": true, "mcpServers": true, "interface": true, "author": true, "homepage": true,
		"repository": true, "license": true, "keywords": true}
	for k := range m {
		if !allowed[k] {
			t.Errorf("plugin.json field %q is not accepted by Codex plugin validation", k)
		}
	}
	if m["skills"] != "./skills/" || m["mcpServers"] != "./.mcp.json" {
		t.Errorf("component paths: skills=%v mcpServers=%v", m["skills"], m["mcpServers"])
	}
	iface, _ := m["interface"].(map[string]any)
	for _, f := range []string{"displayName", "shortDescription", "longDescription", "developerName", "category"} {
		if s, _ := iface[f].(string); s == "" {
			t.Errorf("interface.%s is required and empty", f)
		}
	}
	if caps, _ := iface["capabilities"].([]any); len(caps) == 0 {
		t.Error("interface.capabilities must be a non-empty array")
	}
	if _, ok := iface["defaultPrompt"]; !ok {
		t.Error("interface.defaultPrompt is required")
	}
}

func TestTargetsInterfaceOverrideMergesIntoCard(t *testing.T) {
	p := samplePlugin()
	p.Targets = map[string]map[string]any{name: {"interface": map[string]any{"category": "Coding", "brandColor": "#112233"}}}
	b, _ := compile(t, p)
	var m map[string]any
	_ = json.Unmarshal(b.Files[".codex-plugin/plugin.json"], &m)
	iface := m["interface"].(map[string]any)
	if iface["category"] != "Coding" || iface["brandColor"] != "#112233" || iface["displayName"] != "demo" {
		t.Errorf("override not merged into the derived card: %v", iface)
	}
}

func TestSkillKeepsNameAndDescriptionAndDropsTheRest(t *testing.T) {
	b, ds := compile(t, samplePlugin())
	skill := string(b.Files["skills/deploy/SKILL.md"])
	if !strings.Contains(skill, `name: "deploy"`) || !strings.Contains(skill, "Deploy it When shipping.") {
		t.Errorf("skill frontmatter lost name or folded description:\n%s", skill)
	}
	for _, gone := range []string{"model", "allowed-tools", "disable-model-invocation"} {
		if strings.Contains(skill, gone) {
			t.Errorf("Codex SKILL.md carries %q:\n%s", gone, skill)
		}
	}
	if !hasWarning(ds, "skill:deploy", "tool restrictions, model, autoInvoke: false") {
		t.Errorf("dropped skill fields not reported: %v", ds)
	}
}

func TestCommandBecomesSkillAndAgentsAndGuidanceDegrade(t *testing.T) {
	b, ds := compile(t, samplePlugin())
	if !strings.Contains(string(b.Files["skills/review/SKILL.md"]), `name: "review"`) {
		t.Error("command not emitted as a skill")
	}
	if !hasWarning(ds, "command:review", "no slash commands") {
		t.Error("command degradation not reported")
	}
	if !hasWarning(ds, "agent:rev", "cannot carry subagents") {
		t.Error("dropped agent not reported")
	}
	if !hasWarning(ds, "guidance", "AGENTS.md") {
		t.Error("dropped guidance not reported")
	}
}

func TestHooksUsePluginRootAndDropUndispatchedEvents(t *testing.T) {
	b, ds := compile(t, samplePlugin())
	var h struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct{ Type, Command string }
		}
	}
	if err := json.Unmarshal(b.Files["hooks/hooks.json"], &h); err != nil {
		t.Fatal(err)
	}
	pre := h.Hooks["PreToolUse"]
	if len(pre) != 1 || pre[0].Matcher != "Bash" || pre[0].Hooks[0].Command != "${PLUGIN_ROOT}/hooks/guard.sh" {
		t.Errorf("PreToolUse hook: %+v", pre)
	}
	if _, ok := h.Hooks["Notification"]; ok {
		t.Error("an event Codex does not dispatch was emitted")
	}
	if !hasWarning(ds, "hooks", `"Notification" is not dispatched`) {
		t.Error("dropped hook event not reported")
	}
	if !hasWarning(ds, "hooks", "trusts them") {
		t.Error("hook trust requirement not reported")
	}
}

func TestNoHooksFileWhenEveryEventIsDropped(t *testing.T) {
	p := samplePlugin()
	p.Hooks = []model.Hook{{Event: "Notification", Type: "command", Command: "./hooks/guard.sh"}}
	b, _ := compile(t, p)
	if _, ok := b.Files["hooks/hooks.json"]; ok {
		t.Error("hooks.json emitted with no dispatchable event")
	}
	if _, ok := b.Files["hooks/guard.sh"]; !ok {
		t.Error("bundled hook script dropped with its hooks")
	}
}

// Codex does not expand ${PLUGIN_ROOT} in .mcp.json, and starts a plugin's
// stdio server from the plugin directory only when the server sets cwd to ".":
// measured on Codex CLI 0.155.1, "${PLUGIN_ROOT}/bin/srv" and "./bin/srv" both
// failed with "No such file or directory", and "./bin/srv" with cwd "." started.
func TestMCPRunsBundledServersFromThePluginDirAndDropsSSE(t *testing.T) {
	p := samplePlugin()
	p.MCPServers = append(p.MCPServers, model.MCPServer{Name: "npx", Transport: "stdio", Command: "npx", Args: []string{"-y", "pkg"}})
	b, ds := compile(t, p)
	var m struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(b.Files[".mcp.json"], &m); err != nil {
		t.Fatal(err)
	}
	local := m.MCPServers["local"]
	if local["command"] != "./bin/server" || local["cwd"] != "." {
		t.Errorf("bundled stdio server must keep its relative path and run from the plugin dir: %v", local)
	}
	if args, _ := local["args"].([]any); len(args) != 2 || args[1] != "./cfg.json" {
		t.Errorf("bundled arg must stay relative to the plugin dir: %v", local["args"])
	}
	if _, ok := m.MCPServers["npx"]["cwd"]; ok {
		t.Errorf("a server that names no bundled file must keep the default cwd: %v", m.MCPServers["npx"])
	}
	if m.MCPServers["remote"]["url"] != "https://mcp.example.com/mcp" {
		t.Errorf("http server: %v", m.MCPServers["remote"])
	}
	if _, ok := m.MCPServers["old"]; ok {
		t.Error("SSE server emitted")
	}
	if !hasWarning(ds, "mcp:old", "SSE") {
		t.Error("dropped SSE server not reported")
	}
}

func TestInstallPlanPointsAtMarketplacePluginDirs(t *testing.T) {
	a := &Adapter{}
	p := samplePlugin()
	proj, err := a.InstallPlan(p, adapter.ScopeProject, "/work/repo")
	if err != nil || proj.Root != filepath.Join("/work/repo", "plugins", "demo") {
		t.Errorf("project plan: %+v %v", proj, err)
	}
	user, err := a.InstallPlan(p, adapter.ScopeUser, "/work/repo")
	if err != nil || !strings.HasSuffix(user.Root, filepath.Join("plugins", "demo")) {
		t.Errorf("user plan: %+v %v", user, err)
	}
	if _, err := a.InstallPlan(p, "elsewhere", "/work/repo"); err == nil {
		t.Error("unknown scope accepted")
	}
	// Install writes the plugin directory only; it does not touch a marketplace,
	// so the plan must say what is left to do rather than that it is listed.
	for _, plan := range []adapter.InstallPlan{proj, user} {
		if strings.Contains(plan.Description, "listed in") ||
			!strings.Contains(plan.Description, "marketplace.json") || !strings.Contains(plan.Description, "codex plugin add") {
			t.Errorf("plan must name the registration still to do: %q", plan.Description)
		}
	}
}

// Codex does not substitute ${CLAUDE_PLUGIN_ROOT} in skill text; a skill that
// runs a bundled file through it runs against "/..." there.
func TestSkillNamingClaudePluginRootIsReported(t *testing.T) {
	p := samplePlugin()
	p.Skills[0].Body = "Run `node \"${CLAUDE_PLUGIN_ROOT}/scripts/go.mjs\"`."
	p.Commands[0].Body = "Run $CLAUDE_PLUGIN_ROOT/scripts/review.sh."
	_, ds := compile(t, p)
	if !hasWarning(ds, "skill:deploy", "CLAUDE_PLUGIN_ROOT") {
		t.Error("skill body naming ${CLAUDE_PLUGIN_ROOT} not reported")
	}
	if !hasWarning(ds, "command:review", "CLAUDE_PLUGIN_ROOT") {
		t.Error("command body naming $CLAUDE_PLUGIN_ROOT not reported")
	}
	_, clean := compile(t, samplePlugin())
	for _, d := range clean {
		if strings.Contains(d.Message, "CLAUDE_PLUGIN_ROOT") {
			t.Errorf("reported without the variable in any body: %v", d)
		}
	}
}

func TestRegisteredAsCodex(t *testing.T) {
	if _, ok := adapter.Get("codex"); !ok {
		t.Error("codex adapter not registered")
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Codex passes a plugin server's env values literally. A value that only names
// a variable of the same name is forwarded through env_vars (measured on Codex
// CLI 0.155.1: env {"FOO":"${HUNT_FOO}"} reached the server as the text
// "${HUNT_FOO}", env_vars ["HUNT_FOO"] as its value); anything else that names
// a variable cannot be expressed and is reported.
func TestMCPEnvReferencesUseEnvVarsOrAreReported(t *testing.T) {
	p := samplePlugin()
	p.MCPServers = []model.MCPServer{{Name: "gh", Transport: "stdio", Command: "npx",
		Env: map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}", "API_KEY": "${OTHER}", "MODE": "fast"}}}
	b, ds := compile(t, p)
	var m struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(b.Files[".mcp.json"], &m); err != nil {
		t.Fatal(err)
	}
	gh := m.MCPServers["gh"]
	if vars, _ := gh["env_vars"].([]any); len(vars) != 1 || vars[0] != "GITHUB_TOKEN" {
		t.Errorf("a same-name reference must be forwarded through env_vars: %v", gh)
	}
	env, _ := gh["env"].(map[string]any)
	if _, ok := env["GITHUB_TOKEN"]; ok || env["MODE"] != "fast" {
		t.Errorf("env must keep plain values and drop the forwarded one: %v", env)
	}
	if !hasWarning(ds, "mcp:gh", "API_KEY") {
		t.Error("a reference Codex cannot forward was not reported")
	}
}

func TestHookTimeoutEmittedOnlyWhenSet(t *testing.T) {
	p := samplePlugin()
	p.Hooks = []model.Hook{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "./hooks/guard.sh", Timeout: 30},
		{Event: "SessionStart", Type: "command", Command: "./hooks/guard.sh"},
	}
	b, _ := compile(t, p)
	var h struct {
		Hooks map[string][]struct{ Hooks []map[string]any }
	}
	if err := json.Unmarshal(b.Files["hooks/hooks.json"], &h); err != nil {
		t.Fatal(err)
	}
	if got := h.Hooks["PreToolUse"][0].Hooks[0]["timeout"]; got != float64(30) {
		t.Errorf("PreToolUse timeout = %v, want 30", got)
	}
	if _, ok := h.Hooks["SessionStart"][0].Hooks[0]["timeout"]; ok {
		t.Error("an unset timeout must not be emitted")
	}
}
