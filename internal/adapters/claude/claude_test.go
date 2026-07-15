package claude

import (
	"strings"
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
)

func boolPtr(b bool) *bool { return &b }

func samplePlugin() *model.Plugin {
	return &model.Plugin{
		Name: "demo", Version: "1.2.3", Description: "demo plugin",
		Author:  model.Author{Name: "Ashok"},
		License: "MIT", Homepage: "https://example.com",
		Repository: "https://github.com/x/demo", Keywords: []string{"ai", "demo"},
		Skills: []model.Skill{{
			Name: "deploy", Description: "Deploy it", Model: model.TierBalanced,
			AutoInvoke: boolPtr(false), AllowedTools: []string{"Bash(git push *)", "Read"},
			Body:  "Do the deploy.",
			Files: []model.File{{RelPath: "scripts/go.sh", Content: []byte("echo hi\n")}},
		}},
		Commands: []model.Command{{Name: "review", Description: "Review", Body: "Review it."}},
		Agents: []model.Agent{{
			Name: "rev", Description: "Reviewer", Model: model.TierPowerful,
			Tools: []string{"Read"}, MaxTurns: 5, Color: "blue", Body: "You review.",
		}},
		Hooks:      []model.Hook{{Event: "PostToolUse", Matcher: "Edit", Type: "command", Command: "./hooks/format.sh"}},
		HookFiles:  []model.File{{RelPath: "hooks/format.sh", Content: []byte("echo hi\n")}},
		MCPServers: []model.MCPServer{{Name: "gh", Transport: "stdio", Command: "npx", Args: []string{"-y", "x"}}},
		Guidance:   &model.Guidance{Body: "Be careful."},
	}
}

func compile(t *testing.T) adapter.Bundle {
	t.Helper()
	b, _, err := (&Adapter{}).Compile(samplePlugin())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return b
}

func TestCompileProducesExpectedFiles(t *testing.T) {
	b := compile(t)
	want := []string{
		".claude-plugin/plugin.json",
		"skills/deploy/SKILL.md",
		"skills/deploy/scripts/go.sh",
		"commands/review.md",
		"agents/rev.md",
		"hooks/hooks.json",
		"hooks/format.sh",
		".mcp.json",
		"CLAUDE.md",
	}
	for _, w := range want {
		if _, ok := b.Files[w]; !ok {
			t.Errorf("missing expected file %q", w)
		}
	}
}

func TestManifestMetadataEmitted(t *testing.T) {
	b := compile(t)
	manifest := string(b.Files[".claude-plugin/plugin.json"])
	for _, want := range []string{`"license": "MIT"`, `"homepage": "https://example.com"`,
		`"repository": "https://github.com/x/demo"`, `"keywords"`} {
		if !strings.Contains(manifest, want) {
			t.Errorf("plugin.json missing %s:\n%s", want, manifest)
		}
	}
}

func TestManifestMetadataOmittedWhenEmpty(t *testing.T) {
	p := samplePlugin()
	p.License, p.Homepage, p.Repository, p.Keywords = "", "", "", nil
	b, _, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	manifest := string(b.Files[".claude-plugin/plugin.json"])
	for _, absent := range []string{`"license"`, `"homepage"`, `"repository"`, `"keywords"`} {
		if strings.Contains(manifest, absent) {
			t.Errorf("plugin.json should omit empty %s:\n%s", absent, manifest)
		}
	}
}

func TestHookCommandUsesPluginRoot(t *testing.T) {
	b := compile(t)
	hooks := string(b.Files["hooks/hooks.json"])
	if !strings.Contains(hooks, "${CLAUDE_PLUGIN_ROOT}/hooks/format.sh") {
		t.Errorf("bundled hook command must use ${CLAUDE_PLUGIN_ROOT}:\n%s", hooks)
	}
	if strings.Contains(hooks, `"./hooks/format.sh"`) {
		t.Errorf("relative hook path should have been rewritten:\n%s", hooks)
	}
}

func TestTierMapping(t *testing.T) {
	b := compile(t)
	skill := string(b.Files["skills/deploy/SKILL.md"])
	if !strings.Contains(skill, "model: sonnet") {
		t.Errorf("balanced tier should map to sonnet:\n%s", skill)
	}
	agent := string(b.Files["agents/rev.md"])
	if !strings.Contains(agent, "model: opus") {
		t.Errorf("powerful tier should map to opus:\n%s", agent)
	}
}

func TestAutoInvokeMapping(t *testing.T) {
	b := compile(t)
	skill := string(b.Files["skills/deploy/SKILL.md"])
	if !strings.Contains(skill, "disable-model-invocation: true") {
		t.Errorf("autoInvoke=false should emit disable-model-invocation: true:\n%s", skill)
	}
}

func TestToolPatternPreserved(t *testing.T) {
	b := compile(t)
	skill := string(b.Files["skills/deploy/SKILL.md"])
	if !strings.Contains(skill, `allowed-tools: ["Bash(git push *)", "Read"]`) {
		t.Errorf("tool pattern not preserved:\n%s", skill)
	}
}

func TestHooksWrappedUnderHooksKey(t *testing.T) {
	b := compile(t)
	hooks := string(b.Files["hooks/hooks.json"])
	if !strings.Contains(hooks, `"hooks"`) {
		t.Errorf("plugin hooks.json must wrap events under a \"hooks\" key:\n%s", hooks)
	}
	// The event must be nested, not at the top level.
	if strings.HasPrefix(strings.TrimSpace(hooks), `{`) && !strings.Contains(hooks, `"PostToolUse"`) {
		t.Errorf("expected PostToolUse event in hooks.json:\n%s", hooks)
	}
}

func TestMCPStdioShape(t *testing.T) {
	b := compile(t)
	mcp := string(b.Files[".mcp.json"])
	if !strings.Contains(mcp, `"mcpServers"`) || !strings.Contains(mcp, `"command": "npx"`) {
		t.Errorf("unexpected .mcp.json:\n%s", mcp)
	}
	if !strings.Contains(mcp, `"type": "stdio"`) {
		t.Errorf("stdio server should declare type: stdio:\n%s", mcp)
	}
}

func TestDeterministicCompile(t *testing.T) {
	a := compile(t)
	c := compile(t)
	for k, v := range a.Files {
		if string(c.Files[k]) != string(v) {
			t.Errorf("non-deterministic output for %q", k)
		}
	}
}

func TestCapabilities(t *testing.T) {
	c := (&Adapter{}).Capabilities()
	if !c.Skills || !c.MCP || c.Commands != adapter.CmdNative || !c.Agents || !c.Hooks || !c.Guidance {
		t.Errorf("unexpected capabilities: %+v", c)
	}
}

func TestExecBitPreservedForBundledScript(t *testing.T) {
	p := samplePlugin()
	p.Skills[0].Files[0].Mode = 0o755
	b, _, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if b.Modes["skills/deploy/scripts/go.sh"] != 0o755 {
		t.Errorf("exec bit not preserved: mode = %v", b.Modes["skills/deploy/scripts/go.sh"])
	}
}
