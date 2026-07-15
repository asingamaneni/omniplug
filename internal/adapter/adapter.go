// Package adapter defines the extension seam of omniplug: the Adapter interface
// that every target (Claude Code, Cursor, Codex, ...) implements, plus the
// supporting types (Capabilities, Bundle, Diagnostic) and a registry.
//
// Adding a new target means implementing Adapter and calling Register from the
// adapter package's init(). No changes to the parser, compiler, or CLI.
package adapter

import (
	"io/fs"

	"github.com/asingamaneni/omniplug/internal/model"
)

// Scope is where a compiled plugin is installed.
type Scope string

// Install scopes.
const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

// Severity classifies a Diagnostic.
type Severity string

// Diagnostic severities. Errors block compilation; warnings do not.
const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Diagnostic is a single validation or compilation message.
type Diagnostic struct {
	Severity  Severity
	Source    string // adapter name or "schema"
	Component string // e.g. "skill:deploy" or "hooks"
	Message   string
}

// Warn is a constructor for a warning diagnostic.
func Warn(source, component, msg string) Diagnostic {
	return Diagnostic{Severity: SeverityWarning, Source: source, Component: component, Message: msg}
}

// Error is a constructor for an error diagnostic.
func Error(source, component, msg string) Diagnostic {
	return Diagnostic{Severity: SeverityError, Source: source, Component: component, Message: msg}
}

// HasErrors reports whether any diagnostic in the slice is an error.
func HasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// CommandSupport describes how a target expresses canonical commands.
type CommandSupport string

// How each target expresses canonical commands.
const (
	CmdNative  CommandSupport = "native"  // first-class slash commands
	CmdRules   CommandSupport = "rules"   // mapped to manual rules
	CmdPrompts CommandSupport = "prompts" // mapped to prompt files
	CmdNone    CommandSupport = "none"    // unsupported
)

// Capabilities declares what a target can express. The compiler uses this to
// emit graceful-degradation diagnostics instead of branching on target name.
type Capabilities struct {
	Skills   bool
	MCP      bool
	Commands CommandSupport
	Agents   bool
	Hooks    bool
	Guidance bool
}

// Bundle is the in-memory set of files an adapter produces. Keys are slash-separated
// paths relative to the target's output root. Modes carries non-default file
// modes (only set for files that must be executable, e.g. bundled scripts).
type Bundle struct {
	Files map[string][]byte
	Modes map[string]fs.FileMode
}

// NewBundle returns an empty Bundle.
func NewBundle() Bundle {
	return Bundle{Files: map[string][]byte{}, Modes: map[string]fs.FileMode{}}
}

// Add stores a file's content at the given relative path (default mode).
func (b Bundle) Add(path string, content []byte) { b.Files[path] = content }

// AddFile stores content with an explicit (sanitized) mode. Use for bundled
// supporting files that may need the executable bit preserved.
func (b Bundle) AddFile(path string, content []byte, mode fs.FileMode) {
	b.Files[path] = content
	b.Modes[path] = ScriptMode(mode)
}

// ScriptMode sanitizes a source file mode to a safe output mode: 0o755 when any
// executable bit is set, 0o644 otherwise. Setuid/setgid/sticky bits are never
// propagated, so an untrusted source cannot ship a privileged file.
func ScriptMode(m fs.FileMode) fs.FileMode {
	if m.Perm()&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

// InstallPlan describes where a Bundle is written for a given scope.
type InstallPlan struct {
	Root        string // absolute base directory
	Description string // human-readable summary
}

// Adapter compiles the canonical IR into a single target's on-disk layout.
//
// Compile is pure (model in, files out) so it is trivially golden-testable.
// Install location resolution is a separate method so dry-runs never touch disk.
type Adapter interface {
	// Name is the stable target identifier, e.g. "claude".
	Name() string

	// Capabilities declares what this target supports.
	Capabilities() Capabilities

	// Validate checks target-specific constraints before compilation.
	Validate(p *model.Plugin) []Diagnostic

	// Compile transforms the IR into a Bundle, returning degradation diagnostics.
	Compile(p *model.Plugin) (Bundle, []Diagnostic, error)

	// InstallPlan resolves where the Bundle installs for the given scope.
	// projectDir is the working directory used to resolve project-scoped paths.
	InstallPlan(p *model.Plugin, scope Scope, projectDir string) (InstallPlan, error)
}
