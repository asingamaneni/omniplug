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
}

func TestNativeAgentFile(t *testing.T) {
	b, _ := compile(t)
	agent := string(b.Files[".cursor/agents/rev.md"])
	if !strings.Contains(agent, `name: "rev"`) {
		t.Errorf("agent file missing name:\n%s", agent)
	}
	// Cursor subagents recognize only name/description; invalid fields must not appear.
	for _, bad := range []string{"readonly", "model:", "maxTurns", "color"} {
		if strings.Contains(agent, bad) {
			t.Errorf("agent file contains non-spec field %q:\n%s", bad, agent)
		}
	}
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
