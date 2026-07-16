package cursor

import (
	"strings"
	"testing"

	"github.com/asingamaneni/omniplug/internal/model"
)

// compileP is a helper that compiles a plugin and returns the bundle + diags.
func compileP(t *testing.T, p *model.Plugin) (map[string]string, []string) {
	t.Helper()
	b, ds, err := (&Adapter{}).Compile(p)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	files := map[string]string{}
	for k, v := range b.Files {
		files[k] = string(v)
	}
	var msgs []string
	for _, d := range ds {
		msgs = append(msgs, d.Message)
	}
	return files, msgs
}

func containsMsg(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

// #6/#7: bundled MCP command and args rebase to the installed .cursor/ path.
func TestMCPBundledCommandAndArgsRebaseToCursor(t *testing.T) {
	p := &model.Plugin{Name: "demo", MCPServers: []model.MCPServer{{
		Name: "local", Transport: "stdio", Command: "./hooks/scripts/server.sh",
		Args: []string{"--config", "./hooks/config.json", "${TOKEN}"},
	}}}
	files, _ := compileP(t, p)
	mcp := files[".cursor/mcp.json"]
	if !strings.Contains(mcp, `"${workspaceFolder}/.cursor/hooks/scripts/server.sh"`) {
		t.Errorf("bundled command not rebased under .cursor/:\n%s", mcp)
	}
	if !strings.Contains(mcp, `"${workspaceFolder}/.cursor/hooks/config.json"`) {
		t.Errorf("bundled arg not rebased under .cursor/:\n%s", mcp)
	}
	if strings.Contains(mcp, `"./hooks/`) {
		t.Errorf("literal ./ reference leaked into mcp.json:\n%s", mcp)
	}
	if !strings.Contains(mcp, `${env:TOKEN}`) {
		t.Errorf("env ref in args not interpolated:\n%s", mcp)
	}
}

// Cluster fix: bundled scripts ship even when no hook survives translation.
func TestHookFilesShipWhenNoHookSurvives(t *testing.T) {
	p := &model.Plugin{Name: "demo",
		// Notification has no Cursor event -> compiled hooks.json is nil.
		Hooks:     []model.Hook{{Event: "Notification", Type: "command", Command: "./hooks/x.sh"}},
		HookFiles: []model.File{{RelPath: "hooks/x.sh", Content: []byte("echo\n"), Mode: 0o755}},
	}
	files, _ := compileP(t, p)
	if _, ok := files[".cursor/hooks.json"]; ok {
		t.Errorf("no hook survived, hooks.json should not be emitted")
	}
	if _, ok := files[".cursor/hooks/x.sh"]; !ok {
		t.Errorf("bundled script must still ship even with no surviving hook:\n%v", keys(files))
	}
}

// #10: the per-command targets.cursor escape hatch overrides alwaysApply
// without producing a duplicate YAML key.
func TestCommandTargetsOverrideAlwaysApply(t *testing.T) {
	p := &model.Plugin{Name: "demo", Commands: []model.Command{{
		Name: "deploy", Description: "Deploy", Body: "go",
		Targets: map[string]map[string]any{"cursor": {"alwaysApply": true}},
	}}}
	files, _ := compileP(t, p)
	rule := files[".cursor/rules/deploy.mdc"]
	if strings.Count(rule, "alwaysApply") != 1 {
		t.Errorf("expected exactly one alwaysApply key (no duplicate):\n%s", rule)
	}
	// The escape hatch emits keys JSON-quoted for injection safety; `"k": true`
	// is valid YAML equivalent to `k: true`.
	if !strings.Contains(rule, `"alwaysApply": true`) {
		t.Errorf("targets override should set alwaysApply true:\n%s", rule)
	}
	if strings.Contains(rule, "alwaysApply: false") {
		t.Errorf("default alwaysApply:false should be suppressed by the override:\n%s", rule)
	}
}

// #11: a command's argumentHint drop is diagnosed (not silent).
func TestCommandArgumentHintDiagnosed(t *testing.T) {
	p := &model.Plugin{Name: "demo", Commands: []model.Command{{
		Name: "deploy", Description: "Deploy", ArgumentHint: "[env]", Body: "go",
	}}}
	_, msgs := compileP(t, p)
	if !containsMsg(msgs, "argumentHint") {
		t.Errorf("dropping a command argumentHint must be diagnosed: %v", msgs)
	}
}

// #8: a manifest-level targets.cursor override has no manifest and is diagnosed.
func TestManifestTargetsCursorWarns(t *testing.T) {
	p := &model.Plugin{Name: "demo", Targets: map[string]map[string]any{"cursor": {"k": "v"}}}
	_, msgs := compileP(t, p)
	if !containsMsg(msgs, "manifest-level targets.cursor") {
		t.Errorf("manifest targets.cursor must be diagnosed on Cursor: %v", msgs)
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
