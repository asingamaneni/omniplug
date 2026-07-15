// Package model defines the canonical in-memory representation (IR) of an
// omniplug plugin. Parsing is the only layer that touches source file formats;
// everything downstream (validation, adapters, compiler) operates on this IR.
package model

import "io/fs"

// APIVersion is the schema version the IR currently targets.
const APIVersion = "omniplug/v1"

// ModelTier is an abstract model capability tier. Raw provider model IDs are
// meaningless across tools, so the canonical format uses tiers and each adapter
// maps them to its own model identifiers.
type ModelTier string

const (
	TierUnset    ModelTier = ""
	TierFast     ModelTier = "fast"
	TierBalanced ModelTier = "balanced"
	TierPowerful ModelTier = "powerful"
	TierInherit  ModelTier = "inherit"
)

// ValidModelTiers lists the accepted tier values (excluding the empty default).
var ValidModelTiers = []ModelTier{TierFast, TierBalanced, TierPowerful, TierInherit}

// ValidEfforts lists accepted effort levels (excluding the empty default).
var ValidEfforts = []string{"low", "medium", "high"}

// Plugin is the canonical representation of a plugin source tree.
type Plugin struct {
	APIVersion  string
	Name        string
	Version     string
	Description string
	Author      Author
	License     string

	Skills     []Skill
	Commands   []Command
	Agents     []Agent
	Hooks      []Hook
	HookFiles  []File // bundled hook scripts (paths relative to plugin root)
	MCPServers []MCPServer
	Guidance   *Guidance

	// Targets holds per-target raw overrides (escape hatch), keyed by adapter
	// name. Adapters copy these verbatim into their output.
	Targets map[string]map[string]any
}

// Author identifies the plugin author.
type Author struct {
	Name string
	URL  string
}

// Skill is a portable SKILL.md skill (the cross-tool Agent Skills standard).
type Skill struct {
	Name            string
	Description     string
	WhenToUse       string
	ArgumentHint    string
	Arguments       []string
	AutoInvoke      *bool // nil = default (true)
	UserInvocable   *bool // nil = default (true)
	AllowedTools    []string
	DisallowedTools []string
	Model           ModelTier
	Effort          string
	Globs           []string
	RunInSubagent   bool
	Body            string // markdown content after frontmatter
	Files           []File // supporting files (scripts/, references/, ...)

	Targets map[string]map[string]any
}

// Command is an explicit slash-command/prompt. A thin specialization of a skill.
type Command struct {
	Name         string
	Description  string
	ArgumentHint string
	AllowedTools []string
	Model        ModelTier
	Body         string

	Targets map[string]map[string]any
}

// Agent is a subagent definition. Its Body is the system prompt.
type Agent struct {
	Name            string
	Description     string
	Tools           []string
	DisallowedTools []string
	Model           ModelTier
	MaxTurns        int
	Color           string
	Body            string

	Targets map[string]map[string]any
}

// Hook is a lifecycle hook bound to an event.
type Hook struct {
	Event   string // canonical event name (e.g. PostToolUse)
	Matcher string // tool-name matcher pattern
	Type    string // command|http|mcp_tool|prompt|agent
	Command string // command to run (for type=command)
}

// MCPServer is a neutral MCP server definition.
type MCPServer struct {
	Name      string
	Transport string // stdio|http|sse
	Command   string
	Args      []string
	Env       map[string]string
	URL       string
}

// Guidance is shared agent guidance (maps to AGENTS.md / CLAUDE.md / rules).
type Guidance struct {
	Body string
}

// File is a supporting file carried verbatim, relative to its component dir.
type File struct {
	RelPath string
	Content []byte
	Mode    fs.FileMode
}
