// Package cursor implements the Adapter for Cursor. It compiles the canonical IR
// into a .cursor/ layout:
//
//	.cursor/skills/<name>/SKILL.md       (portable Agent Skills standard)
//	.cursor/rules/<name>.mdc             (commands -> on-demand rules)
//	.cursor/rules/<plugin>-guidance.mdc  (guidance -> Always rule)
//	.cursor/agents/<name>.md             (native subagents, Cursor 1.7+)
//	.cursor/hooks.json                   (native lifecycle hooks, Cursor 1.7+)
//	.cursor/mcp.json
//
// Cursor governs an agent's tool allowlist via a `readonly` flag rather than an
// explicit list, and supports a narrower model vocabulary, so those fields
// degrade with diagnostics.
package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
	"github.com/asingamaneni/omniplug/internal/yamlfm"
)

const name = "cursor"

func init() { adapter.Register(&Adapter{}) }

// Adapter is the Cursor target.
type Adapter struct{}

// Name returns the stable target identifier.
func (a *Adapter) Name() string { return name }

// Capabilities declares Cursor's support. Cursor has native skills, MCP,
// subagents, and hooks; commands map to on-demand rules.
func (a *Adapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{
		Skills:   true,
		MCP:      true,
		Commands: adapter.CmdRules,
		Agents:   true,
		Hooks:    true,
		Guidance: true,
	}
}

// modelTier maps an abstract tier to a Cursor model alias. Cursor's documented
// vocabulary is "inherit" and "fast"; finer tiers have no alias and are omitted.
func modelTier(t model.ModelTier) (alias string, expressible bool) {
	switch t {
	case model.TierFast:
		return "fast", true
	case model.TierInherit:
		return "inherit", true
	case model.TierUnset:
		return "", true
	default:
		return "", false
	}
}

// Validate checks Cursor-specific constraints.
func (a *Adapter) Validate(p *model.Plugin) []adapter.Diagnostic {
	var ds []adapter.Diagnostic
	if len(p.MCPServers) > 40 {
		ds = append(ds, adapter.Warn(name, "mcp",
			fmt.Sprintf("%d MCP servers configured; Cursor degrades past ~40 active tools", len(p.MCPServers))))
	}
	return ds
}

// Compile transforms the IR into a Cursor .cursor/ bundle.
func (a *Adapter) Compile(p *model.Plugin) (adapter.Bundle, []adapter.Diagnostic, error) {
	b := adapter.NewBundle()
	var ds []adapter.Diagnostic

	for _, s := range p.Skills {
		skill, sd := compileSkill(s)
		b.Add(rel("skills", s.Name, "SKILL.md"), skill)
		ds = append(ds, sd...)
		for _, f := range s.Files {
			b.AddFile(rel("skills", s.Name, f.RelPath), f.Content, f.Mode)
		}
		if len(s.AllowedTools) > 0 || len(s.DisallowedTools) > 0 {
			ds = append(ds, adapter.Warn(name, "skill:"+s.Name,
				"tool restrictions dropped (Cursor governs tools outside SKILL.md)"))
		}
	}

	for _, c := range p.Commands {
		b.Add(rel("rules", c.Name+".mdc"), onDemandRule(c.Description, c.Body))
		if len(c.AllowedTools) > 0 || c.Model != model.TierUnset {
			ds = append(ds, adapter.Warn(name, "command:"+c.Name,
				"allowed-tools/model dropped (not expressible in a Cursor rule)"))
		}
	}

	for _, ag := range p.Agents {
		agent, ad := compileAgent(ag)
		b.Add(rel("agents", ag.Name+".md"), agent)
		ds = append(ds, ad...)
	}

	if len(p.Hooks) > 0 {
		hb, hd := compileHooks(p.Hooks)
		if hb != nil {
			b.Add(rel("hooks.json"), hb)
			for _, f := range p.HookFiles {
				b.AddFile(rel(f.RelPath), f.Content, f.Mode)
			}
		}
		ds = append(ds, hd...)
	}

	if len(p.MCPServers) > 0 {
		mb, err := compileMCP(p.MCPServers)
		if err != nil {
			return b, ds, err
		}
		b.Add(rel("mcp.json"), mb)
	}

	if p.Guidance != nil && p.Guidance.Body != "" {
		b.Add(rel("rules", p.Name+"-guidance.mdc"), alwaysRule(p.Name+" project guidance", p.Guidance.Body))
	}

	return b, ds, nil
}

// InstallPlan resolves the install root for the given scope. Cursor reads from
// .cursor/ at the project root or ~/.cursor globally; this adapter writes the
// .cursor/ subtree, so the install root is the directory that contains it.
func (a *Adapter) InstallPlan(p *model.Plugin, scope adapter.Scope, projectDir string) (adapter.InstallPlan, error) {
	switch scope {
	case adapter.ScopeProject:
		return adapter.InstallPlan{Root: projectDir, Description: "project .cursor/ in " + projectDir}, nil
	case adapter.ScopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return adapter.InstallPlan{}, err
		}
		return adapter.InstallPlan{Root: home, Description: "user .cursor/ in home directory"}, nil
	default:
		return adapter.InstallPlan{}, fmt.Errorf("unknown scope %q", scope)
	}
}

// ---- compilers ----

func compileSkill(s model.Skill) ([]byte, []adapter.Diagnostic) {
	var ds []adapter.Diagnostic
	b := &yamlfm.Builder{}
	b.Scalar("name", s.Name)
	b.Scalar("description", foldDescription(s.Description, s.WhenToUse))
	if alias, ok := modelTier(s.Model); !ok {
		ds = append(ds, adapter.Warn(name, "skill:"+s.Name,
			fmt.Sprintf("model tier %q has no Cursor alias; omitted", s.Model)))
	} else {
		b.Raw("model", alias)
	}
	if s.AutoInvoke != nil && !*s.AutoInvoke {
		b.Bool("disable-model-invocation", true)
	}
	b.Targets(s.Targets[name])

	// Cursor SKILL.md frontmatter supports name/description/model/
	// disable-model-invocation; everything else degrades with a diagnostic.
	var dropped []string
	if s.Effort != "" {
		dropped = append(dropped, "effort")
	}
	if s.ArgumentHint != "" {
		dropped = append(dropped, "argumentHint")
	}
	if len(s.Arguments) > 0 {
		dropped = append(dropped, "arguments")
	}
	if len(s.Globs) > 0 {
		dropped = append(dropped, "globs")
	}
	if s.RunInSubagent {
		dropped = append(dropped, "runInSubagent")
	}
	if s.UserInvocable != nil && !*s.UserInvocable {
		dropped = append(dropped, "userInvocable")
	}
	if len(dropped) > 0 {
		ds = append(ds, adapter.Warn(name, "skill:"+s.Name,
			"Cursor SKILL.md does not support "+strings.Join(dropped, ", ")+"; dropped"))
	}
	return b.Render(s.Body), ds
}

func compileAgent(ag model.Agent) ([]byte, []adapter.Diagnostic) {
	var ds []adapter.Diagnostic
	b := &yamlfm.Builder{}
	// Cursor subagent frontmatter (1.7+): name, description, model, readonly,
	// is_background. The name must match the filename stem; the markdown body
	// is the system prompt.
	b.Scalar("name", ag.Name)
	b.Scalar("description", ag.Description)
	if alias, ok := modelTier(ag.Model); !ok {
		ds = append(ds, adapter.Warn(name, "agent:"+ag.Name,
			fmt.Sprintf("model tier %q has no Cursor alias; subagent inherits the parent model", ag.Model)))
	} else {
		b.Raw("model", alias)
	}
	readonly := agentReadonly(ag)
	if readonly {
		b.Bool("readonly", true)
	}
	b.Targets(ag.Targets[name])

	var dropped []string
	if len(ag.Tools) > 0 || len(ag.DisallowedTools) > 0 {
		if readonly {
			// readonly is at least as restrictive as the canonical config, so
			// this degrades toward safety rather than silently widening access.
			ds = append(ds, adapter.Warn(name, "agent:"+ag.Name,
				"tool restrictions approximated by readonly: true (Cursor has no per-tool lists)"))
		} else {
			dropped = append(dropped, "tools/disallowedTools")
		}
	}
	if ag.MaxTurns > 0 {
		dropped = append(dropped, "maxTurns")
	}
	if ag.Color != "" {
		dropped = append(dropped, "color")
	}
	if len(dropped) > 0 {
		ds = append(ds, adapter.Warn(name, "agent:"+ag.Name,
			"Cursor subagents cannot express "+strings.Join(dropped, ", ")+"; dropped"))
	}
	return b.Render(ag.Body), ds
}

// writeTools are canonical tool names that grant write or exec power.
var writeTools = map[string]bool{
	"Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true, "Bash": true,
}

// agentReadonly reports whether the agent's canonical tool config denies
// writes: either DisallowedTools covers Write and Edit, or an explicit Tools
// allowlist grants no write/exec tool. Cursor's readonly flag is at least as
// restrictive as either form, so mapping to it never widens access.
func agentReadonly(ag model.Agent) bool {
	denied := map[string]bool{}
	for _, t := range ag.DisallowedTools {
		denied[t] = true
	}
	if denied["Write"] && denied["Edit"] {
		return true
	}
	if len(ag.Tools) == 0 {
		return false
	}
	for _, t := range ag.Tools {
		// A pattern like Bash(git push *) still grants exec power.
		base := t
		if i := strings.IndexByte(base, '('); i >= 0 {
			base = base[:i]
		}
		if writeTools[base] {
			return false
		}
	}
	return true
}

// cursorHookEvent maps a canonical (Claude-style) event to a Cursor hooks.json
// v1 event. matcherOK reports whether Cursor matches tool types on that event.
// Unknown events return ok=false. Event names verified against
// https://cursor.com/docs/agent/hooks (hooks.json version 1, July 2026).
func cursorHookEvent(e string) (event string, matcherOK, ok bool) {
	switch e {
	case "PreToolUse":
		return "preToolUse", true, true
	case "PostToolUse":
		return "postToolUse", true, true
	// Cursor matches subagent *types* (e.g. explore|shell) on subagent events;
	// canonical matchers carry Claude agent names, which are untranslatable.
	case "SubagentStart":
		return "subagentStart", false, true
	case "SubagentStop":
		return "subagentStop", false, true
	case "Stop":
		return "stop", false, true
	case "SessionStart":
		return "sessionStart", false, true
	case "SessionEnd":
		return "sessionEnd", false, true
	case "PreCompact":
		return "preCompact", false, true
	case "UserPromptSubmit":
		return "beforeSubmitPrompt", false, true
	default:
		return "", false, false
	}
}

// cursorToolTypes translates Claude tool names into Cursor's matcher
// vocabulary (tool types). Claude's edit-family tools all surface as Cursor
// "Write" operations.
var cursorToolTypes = map[string]string{
	"Bash":         "Shell",
	"Edit":         "Write",
	"Write":        "Write",
	"MultiEdit":    "Write",
	"NotebookEdit": "Write",
	"Read":         "Read",
	"Grep":         "Grep",
	"Task":         "Task",
}

// cursorMatcher translates a Claude tool-name alternation ("Edit|Write") into
// Cursor's tool-type vocabulary ("Write"). dropped lists untranslatable
// tokens (MCP tools, Bash(...) patterns, unknown names).
func cursorMatcher(m string) (translated string, dropped []string) {
	var out []string
	seen := map[string]bool{}
	for _, tok := range strings.Split(m, "|") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if t, ok := cursorToolTypes[tok]; ok {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
			continue
		}
		dropped = append(dropped, tok)
	}
	return strings.Join(out, "|"), dropped
}

func compileHooks(hooks []model.Hook) ([]byte, []adapter.Diagnostic) {
	var ds []adapter.Diagnostic
	type entry struct {
		Command string `json:"command"`
		Matcher string `json:"matcher,omitempty"`
	}
	byEvent := map[string][]entry{}
	for _, h := range hooks {
		ev, matcherOK, ok := cursorHookEvent(h.Event)
		if !ok {
			ds = append(ds, adapter.Warn(name, "hooks",
				fmt.Sprintf("event %q has no Cursor equivalent; dropped", h.Event)))
			continue
		}
		if h.Type != "" && h.Type != "command" {
			ds = append(ds, adapter.Warn(name, "hooks",
				fmt.Sprintf("hook type %q unsupported; Cursor hooks are command processes", h.Type)))
			continue
		}
		// A hook whose matcher cannot be expressed still ships: firing more
		// broadly with a diagnostic beats silently disabling it.
		matcher := ""
		if h.Matcher != "" {
			if !matcherOK {
				ds = append(ds, adapter.Warn(name, "hooks", fmt.Sprintf(
					"matcher %q dropped for event %q (Cursor does not match tool names on %s); hook fires unfiltered — filter inside the script via the stdin JSON",
					h.Matcher, h.Event, ev)))
			} else {
				var droppedToks []string
				matcher, droppedToks = cursorMatcher(h.Matcher)
				if len(droppedToks) > 0 && matcher == "" {
					ds = append(ds, adapter.Warn(name, "hooks", fmt.Sprintf(
						"matcher %q has no Cursor tool-type equivalent; hook fires on every %s — filter inside the script via stdin tool_name",
						h.Matcher, ev)))
				} else if len(droppedToks) > 0 {
					ds = append(ds, adapter.Warn(name, "hooks", fmt.Sprintf(
						"matcher token(s) %s untranslatable to Cursor tool types; hook may fire more broadly — filter inside the script via stdin tool_name",
						strings.Join(droppedToks, ", "))))
				}
			}
		}
		byEvent[ev] = append(byEvent[ev], entry{Command: cursorCommand(h.Command), Matcher: matcher})
	}
	if len(byEvent) == 0 {
		return nil, ds
	}
	doc := map[string]interface{}{"version": 1, "hooks": byEvent}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		ds = append(ds, adapter.Warn(name, "hooks", "failed to encode hooks.json"))
		return nil, ds
	}
	return append(out, '\n'), ds
}

// onDemandRule emits a Cursor rule activated on demand (via @mention or by the
// agent based on its description). globs is empty, alwaysApply false.
func onDemandRule(description, body string) []byte {
	b := &yamlfm.Builder{}
	b.Scalar("description", description)
	b.RawField("globs", "")
	b.Bool("alwaysApply", false)
	return b.Render(body)
}

// alwaysRule emits a rule injected into every prompt.
func alwaysRule(description, body string) []byte {
	b := &yamlfm.Builder{}
	b.Scalar("description", description)
	b.Bool("alwaysApply", true)
	return b.Render(body)
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
			// Cursor infers remote transport from the presence of `url`.
			sj.URL = cursorInterpolate(s.URL)
		default:
			// Cursor marks `type` required for stdio servers.
			sj.Type = "stdio"
			sj.Command = cursorMCPCommand(s.Command)
			sj.Args = cursorInterpolateArgs(s.Args)
			sj.Env = cursorInterpolateEnv(s.Env)
		}
		m[s.Name] = sj
	}
	out, err := json.MarshalIndent(map[string]map[string]serverJSON{"mcpServers": m}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// cursorBuiltins are Cursor's interpolation variables that must NOT be rewritten
// to the env: form.
var cursorBuiltins = map[string]bool{
	"workspaceFolder": true, "workspaceFolderBasename": true,
	"userHome": true, "pathSeparator": true, "/": true,
}

// envRefRe matches a simple ${NAME} reference (a bare identifier).
var envRefRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// cursorInterpolate rewrites Claude/standard `${VAR}` environment references to
// Cursor's required `${env:VAR}` form, leaving Cursor builtins and already
// `env:`-prefixed or defaulted refs untouched.
func cursorInterpolate(s string) string {
	return envRefRe.ReplaceAllStringFunc(s, func(match string) string {
		nameVar := match[2 : len(match)-1]
		if cursorBuiltins[nameVar] {
			return match
		}
		return "${env:" + nameVar + "}"
	})
}

func cursorInterpolateArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = cursorInterpolate(a)
	}
	return out
}

func cursorInterpolateEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = cursorInterpolate(v)
	}
	return out
}

// cursorMCPCommand rewrites a bundled-server command (`./servers/x`) to use
// Cursor's `${workspaceFolder}` root; PATH-resolved commands pass through.
func cursorMCPCommand(cmd string) string {
	if strings.HasPrefix(cmd, "./") {
		return "${workspaceFolder}/" + cmd[len("./"):]
	}
	return cursorInterpolate(cmd)
}

// cursorCommand rewrites a plugin-root-relative command (`./hooks/x.sh`) to a
// project-root-relative path under .cursor/, where the bundled script is emitted.
// Cursor resolves project hook command paths relative to the project root.
func cursorCommand(cmd string) string {
	if strings.HasPrefix(cmd, "./") {
		return ".cursor/" + cmd[len("./"):]
	}
	return cmd
}

// foldDescription appends the whenToUse hint to the description, since Cursor
// has no separate field for it.
func foldDescription(desc, whenToUse string) string {
	if whenToUse == "" {
		return desc
	}
	if desc == "" {
		return whenToUse
	}
	return desc + " " + whenToUse
}

// rel joins path elements under the .cursor/ root with forward slashes.
func rel(parts ...string) string {
	all := append([]string{".cursor"}, parts...)
	return filepath.ToSlash(filepath.Join(all...))
}
