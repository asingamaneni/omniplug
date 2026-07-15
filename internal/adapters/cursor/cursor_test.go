package cursor

import (
	"strings"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
)

func samplePlugin() *model.Plugin {
	return &model.Plugin{
		Name: "demo", Version: "1.0.0", Description: "demo",
		Skills: []model.Skill{{
			Name: "sum", Description: "Summarize", WhenToUse: "When asked what changed",
			Model: model.TierBalanced, AllowedTools: []string{"Read"}, Body: "Summarize.",
		}},
		Commands:  []model.Command{{Name: "deploy", Description: "Deploy", AllowedTools: []string{"Bash(git push *)"}, Body: "Deploy it."}},
		Agents:    []model.Agent{{Name: "rev", Description: "Reviewer", Tools: []string{"Read"}, Body: "Review."}},
		Hooks:     []model.Hook{{Event: "PostToolUse", Matcher: "Edit", Type: "command", Command: "./hooks/format.sh"}},
		HookFiles: []model.File{{RelPath: "hooks/format.sh", Content: []byte("echo hi\n")}},
		MCPServers: []model.MCPServer{
			{Name: "gh", Transport: "stdio", Command: "npx", Args: []string{"-y", "x"}, Env: map[string]string{"TOKEN": "${TOKEN}"}},
			{Name: "docs", Transport: "http", URL: "https://x/docs"},
		},
		Guidance: &model.Guidance{Body: "Be careful."},
	}
}

func compile(t *testing.T) (adapter.Bundle, []adapter.Diagnostic) {
	t.Helper()
	b, ds, err := (&Adapter{}).Compile(samplePlugin())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return b, ds
}

func TestExpectedFiles(t *testing.T) {
	b, _ := compile(t)
	want := []string{
		".cursor/skills/sum/SKILL.md",
		".cursor/rules/deploy.mdc",
		".cursor/agents/rev.md",
		".cursor/rules/demo-guidance.mdc",
		".cursor/hooks.json",
		".cursor/mcp.json",
	}
	for _, w := range want {
		if _, ok := b.Files[w]; !ok {
			t.Errorf("missing expected file %q", w)
		}
	}
}

func TestNativeHooksFile(t *testing.T) {
	b, _ := compile(t)
	hooks := string(b.Files[".cursor/hooks.json"])
	if !strings.Contains(hooks, `"version": 1`) {
		t.Errorf("hooks.json missing version:\n%s", hooks)
	}
	if !strings.Contains(hooks, "postToolUse") {
		t.Errorf("event not mapped to camelCase postToolUse:\n%s", hooks)
	}
	// The Claude tool name "Edit" must be translated to Cursor's "Write" type.
	if !strings.Contains(hooks, `"matcher": "Write"`) {
		t.Errorf("matcher not translated to Cursor tool-type vocabulary:\n%s", hooks)
	}
	if strings.Contains(hooks, `"matcher": "Edit"`) {
		t.Errorf("Claude tool name leaked into Cursor matcher:\n%s", hooks)
	}
}

func TestHookEventMapping(t *testing.T) {
	p := &model.Plugin{Name: "demo", Hooks: []model.Hook{
		{Event: "Stop", Type: "command", Command: "./hooks/a.sh"},
		{Event: "UserPromptSubmit", Type: "command", Command: "./hooks/b.sh"},
		{Event: "SessionStart", Type: "command", Command: "./hooks/c.sh"},
		{Event: "PreCompact", Type: "command", Command: "./hooks/d.sh"},
		{Event: "Notification", Type: "command", Command: "./hooks/e.sh"},
	}}
	b, ds, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	hooks := string(b.Files[".cursor/hooks.json"])
	for _, want := range []string{`"stop"`, `"beforeSubmitPrompt"`, `"sessionStart"`, `"preCompact"`} {
		if !strings.Contains(hooks, want) {
			t.Errorf("hooks.json missing event %s:\n%s", want, hooks)
		}
	}
	if strings.Contains(hooks, "notification") || strings.Contains(hooks, "Notification") {
		t.Errorf("Notification has no Cursor equivalent and must be dropped:\n%s", hooks)
	}
	if !hasWarnContaining(ds, "Notification") {
		t.Errorf("dropping Notification must produce a diagnostic: %+v", ds)
	}
}

func TestHookMatcherTranslation(t *testing.T) {
	p := &model.Plugin{Name: "demo", Hooks: []model.Hook{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "./hooks/a.sh"},
		{Event: "PreToolUse", Matcher: "Bash(git push *)", Type: "command", Command: "./hooks/b.sh"},
		{Event: "SubagentStop", Matcher: "rev", Type: "command", Command: "./hooks/c.sh"},
	}}
	b, ds, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	hooks := string(b.Files[".cursor/hooks.json"])
	if !strings.Contains(hooks, `"matcher": "Shell"`) {
		t.Errorf("Bash should translate to Shell:\n%s", hooks)
	}
	// Untranslatable matchers are omitted but the hooks still ship.
	if strings.Contains(hooks, "git push") || strings.Contains(hooks, `"rev"`) {
		t.Errorf("untranslatable matcher leaked into output:\n%s", hooks)
	}
	if !strings.Contains(hooks, ".cursor/hooks/b.sh") || !strings.Contains(hooks, ".cursor/hooks/c.sh") {
		t.Errorf("hooks with untranslatable matchers must still be emitted:\n%s", hooks)
	}
	if !hasWarnContaining(ds, "Bash(git push *)") || !hasWarnContaining(ds, `"rev"`) {
		t.Errorf("untranslatable matchers must produce diagnostics: %+v", ds)
	}
}

// TestHookMatcherPartialTranslationFiresUnfiltered locks the safe-degradation
// rule: a matcher mixing translatable and untranslatable tokens must NOT emit
// the translated subset (which would silently skip the dropped tools); it fires
// unfiltered so a guard hook still runs on every operation.
func TestHookMatcherPartialTranslationFiresUnfiltered(t *testing.T) {
	p := &model.Plugin{Name: "demo", Hooks: []model.Hook{
		{Event: "PreToolUse", Matcher: "Bash|mcp__github", Type: "command", Command: "./hooks/guard.sh"},
	}}
	b, ds, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	hooks := string(b.Files[".cursor/hooks.json"])
	if strings.Contains(hooks, `"matcher"`) {
		t.Errorf("a partially-translatable matcher must be dropped, not narrowed:\n%s", hooks)
	}
	if !strings.Contains(hooks, ".cursor/hooks/guard.sh") {
		t.Errorf("the hook must still ship (fires unfiltered):\n%s", hooks)
	}
	if !hasWarnContaining(ds, "mcp__github") || !hasWarnContaining(ds, "unfiltered") {
		t.Errorf("partial translation must warn that the hook fires unfiltered: %+v", ds)
	}
}

func TestExecBitPreservedForBundledHookScript(t *testing.T) {
	p := &model.Plugin{Name: "demo",
		Hooks:     []model.Hook{{Event: "PostToolUse", Type: "command", Command: "./hooks/format.sh"}},
		HookFiles: []model.File{{RelPath: "hooks/format.sh", Content: []byte("echo hi\n"), Mode: 0o755}},
	}
	b, _, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if b.Modes[".cursor/hooks/format.sh"] != 0o755 {
		t.Errorf("exec bit not preserved on bundled hook script: mode = %v", b.Modes[".cursor/hooks/format.sh"])
	}
}

func TestNativeAgentFile(t *testing.T) {
	b, ds := compile(t)
	agent := string(b.Files[".cursor/agents/rev.md"])
	if !strings.Contains(agent, `name: "rev"`) {
		t.Errorf("agent file missing name:\n%s", agent)
	}
	// A Read-only allowlist maps to Cursor's readonly flag.
	if !strings.Contains(agent, "readonly: true") {
		t.Errorf("read-only tool allowlist should emit readonly: true:\n%s", agent)
	}
	if !hasWarnContaining(ds, "readonly") {
		t.Errorf("readonly approximation must be diagnosed: %+v", ds)
	}
	// No model tier set -> no model key; maxTurns/color unset.
	for _, bad := range []string{"model:", "maxTurns", "color"} {
		if strings.Contains(agent, bad) {
			t.Errorf("agent file contains unexpected field %q:\n%s", bad, agent)
		}
	}
}

func TestAgentTiers(t *testing.T) {
	p := &model.Plugin{Name: "demo", Agents: []model.Agent{
		{Name: "fast", Description: "d", Model: model.TierFast, Body: "x"},
		{Name: "inh", Description: "d", Model: model.TierInherit, Body: "x"},
		{Name: "pow", Description: "d", Model: model.TierPowerful, Body: "x"},
	}}
	b, ds, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(string(b.Files[".cursor/agents/fast.md"]), "model: fast") {
		t.Errorf("TierFast should emit model: fast:\n%s", b.Files[".cursor/agents/fast.md"])
	}
	if !strings.Contains(string(b.Files[".cursor/agents/inh.md"]), "model: inherit") {
		t.Errorf("TierInherit should emit model: inherit:\n%s", b.Files[".cursor/agents/inh.md"])
	}
	if strings.Contains(string(b.Files[".cursor/agents/pow.md"]), "model:") {
		t.Errorf("TierPowerful has no Cursor alias and must be omitted:\n%s", b.Files[".cursor/agents/pow.md"])
	}
	if !hasWarnContaining(ds, "powerful") {
		t.Errorf("omitting the powerful tier must be diagnosed: %+v", ds)
	}
}

func TestAgentReadonlyDerivation(t *testing.T) {
	cases := []struct {
		name     string
		agent    model.Agent
		readonly bool
	}{
		{"denies write+edit", model.Agent{Name: "a", Description: "d", DisallowedTools: []string{"Write", "Edit"}}, true},
		{"read-only allowlist", model.Agent{Name: "b", Description: "d", Tools: []string{"Read", "Grep"}}, true},
		{"allowlist grants bash pattern", model.Agent{Name: "c", Description: "d", Tools: []string{"Read", "Bash(git push *)"}}, false},
		{"no tool config", model.Agent{Name: "d", Description: "d"}, false},
		{"denies only write", model.Agent{Name: "e", Description: "d", DisallowedTools: []string{"Write"}}, false},
	}
	for _, tc := range cases {
		if got := agentReadonly(tc.agent); got != tc.readonly {
			t.Errorf("%s: agentReadonly = %v, want %v", tc.name, got, tc.readonly)
		}
	}
}

func TestSkillDroppedFieldsDiagnosed(t *testing.T) {
	p := &model.Plugin{Name: "demo", Skills: []model.Skill{{
		Name: "s", Description: "d", Effort: "high", ArgumentHint: "<x>", Globs: []string{"*.go"},
		Body: "x",
	}}}
	b, ds, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	skill := string(b.Files[".cursor/skills/s/SKILL.md"])
	for _, bad := range []string{"effort", "argument", "globs"} {
		if strings.Contains(skill, bad) {
			t.Errorf("unsupported field %q leaked into Cursor SKILL.md:\n%s", bad, skill)
		}
	}
	if !hasWarnContaining(ds, "effort") || !hasWarnContaining(ds, "argumentHint") || !hasWarnContaining(ds, "globs") {
		t.Errorf("dropped skill fields must be diagnosed: %+v", ds)
	}
}

func hasWarnContaining(ds []adapter.Diagnostic, sub string) bool {
	for _, d := range ds {
		if strings.Contains(d.Message, sub) {
			return true
		}
	}
	return false
}

func TestHookScriptBundledAndPathRewritten(t *testing.T) {
	b, _ := compile(t)
	if _, ok := b.Files[".cursor/hooks/format.sh"]; !ok {
		t.Error("hook script not bundled under .cursor/hooks/")
	}
	hooks := string(b.Files[".cursor/hooks.json"])
	if !strings.Contains(hooks, ".cursor/hooks/format.sh") {
		t.Errorf("hook command not rewritten to bundled path:\n%s", hooks)
	}
}

func TestManualRuleShape(t *testing.T) {
	b, _ := compile(t)
	rule := string(b.Files[".cursor/rules/deploy.mdc"])
	if !strings.Contains(rule, "alwaysApply: false") || !strings.Contains(rule, "globs:") {
		t.Errorf("manual rule missing expected frontmatter:\n%s", rule)
	}
}

func TestGuidanceIsAlwaysRule(t *testing.T) {
	b, _ := compile(t)
	rule := string(b.Files[".cursor/rules/demo-guidance.mdc"])
	if !strings.Contains(rule, "alwaysApply: true") {
		t.Errorf("guidance should be an Always rule:\n%s", rule)
	}
}

func TestWhenToUseFolded(t *testing.T) {
	b, _ := compile(t)
	skill := string(b.Files[".cursor/skills/sum/SKILL.md"])
	if !strings.Contains(skill, "When asked what changed") {
		t.Errorf("whenToUse not folded into description:\n%s", skill)
	}
}

func TestMCPHTTPUsesURL(t *testing.T) {
	b, _ := compile(t)
	mcp := string(b.Files[".cursor/mcp.json"])
	if !strings.Contains(mcp, `"url": "https://x/docs"`) {
		t.Errorf("http server should use url:\n%s", mcp)
	}
}

func TestMCPStdioRequiresType(t *testing.T) {
	b, _ := compile(t)
	mcp := string(b.Files[".cursor/mcp.json"])
	if !strings.Contains(mcp, `"type": "stdio"`) {
		t.Errorf("Cursor stdio server must declare type: stdio:\n%s", mcp)
	}
}

func TestMCPEnvInterpolationPrefixed(t *testing.T) {
	b, _ := compile(t)
	mcp := string(b.Files[".cursor/mcp.json"])
	if !strings.Contains(mcp, `${env:TOKEN}`) {
		t.Errorf("Cursor env refs must use ${env:VAR} form:\n%s", mcp)
	}
	if strings.Contains(mcp, `"${TOKEN}"`) {
		t.Errorf("bare ${VAR} should have been rewritten for Cursor:\n%s", mcp)
	}
}

func TestInterpolationLeavesBuiltinsAlone(t *testing.T) {
	if got := cursorInterpolate("${workspaceFolder}/x"); got != "${workspaceFolder}/x" {
		t.Errorf("builtin rewritten: %q", got)
	}
	if got := cursorInterpolate("${API_KEY}"); got != "${env:API_KEY}" {
		t.Errorf("env ref = %q, want ${env:API_KEY}", got)
	}
}

func TestCapabilities(t *testing.T) {
	c := (&Adapter{}).Capabilities()
	if !c.Skills || !c.MCP || c.Commands != adapter.CmdRules || !c.Agents || !c.Hooks || !c.Guidance {
		t.Errorf("unexpected capabilities: %+v", c)
	}
}

func TestDeterministicCompile(t *testing.T) {
	a, _ := compile(t)
	c, _ := compile(t)
	if len(a.Files) != len(c.Files) {
		t.Fatalf("file count differs: %d vs %d", len(a.Files), len(c.Files))
	}
	for k, v := range a.Files {
		if string(c.Files[k]) != string(v) {
			t.Errorf("non-deterministic output for %q", k)
		}
	}
}
