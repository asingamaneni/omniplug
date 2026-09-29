# omniplug

Author an AI agent plugin **once** in a tool-neutral canonical format, then compile or install it into target-specific layouts. Claude Code, Cursor and Codex are supported today; future tools (Grok, Gemini CLI, …) slot in by implementing one adapter — no changes to the core.

Docs: **[asingamaneni.github.io/omniplug](https://asingamaneni.github.io/omniplug/)** — [Installation](https://asingamaneni.github.io/omniplug/docs/installation/) · [Usage](https://asingamaneni.github.io/omniplug/docs/usage/) · [Architecture](https://asingamaneni.github.io/omniplug/docs/architecture/)

## Status

End-to-end pipeline (parse → IR → validate → compile → install) with three target adapters:

| Target | Skills | MCP | Commands | Agents | Hooks | Guidance |
| ------ | :----: | :-: | :------: | :----: | :---: | :------: |
| **claude** | yes | yes | native | yes | yes | yes |
| **cursor** | yes | yes | rules | yes | yes | yes |
| **codex** | yes | yes | none (as skills) | no | yes | no |

Both targets support every component natively. Where a canonical field has no native home, the adapter degrades it with a diagnostic instead of producing incorrect output — e.g. hook matchers are translated from Claude tool names to Cursor tool types (`Bash`→`Shell`, `Edit`→`Write`), a write-denying agent tool config becomes Cursor's `readonly: true`, and untranslatable matchers ship unfiltered with a warning rather than silently never firing. **Codex** compiles to a Codex plugin (`.codex-plugin/plugin.json`, `skills/`, `hooks/hooks.json`, `.mcp.json`), the unit `codex plugin add` installs from a marketplace. A Codex plugin has no slash commands, subagents or guidance file, so commands become skills and agents and guidance are dropped, each with a warning; hook scripts are rewritten to `${PLUGIN_ROOT}`, a stdio MCP server that runs a bundled file keeps its relative path and gets `cwd: "."` (Codex resolves it against the plugin directory and does not expand `${PLUGIN_ROOT}` in `.mcp.json`), an env value `${NAME}` for its own name is forwarded through `env_vars` (Codex passes env values as literal text, so any other `${...}` in one is reported), a skill that names `$CLAUDE_PLUGIN_ROOT` is reported, and events Codex does not dispatch are dropped with a warning. Codex runs a plugin's hooks only after the user trusts them, which `build` reports.

Codex output passes the plugin validator that ships inside Codex CLI 0.155.1. Output formats were validated against the official Claude Code and Cursor documentation (July 2026): plugin `hooks.json` wrapping and `${CLAUDE_PLUGIN_ROOT}` rewriting, `.mcp.json` shapes, Cursor `hooks.json` v1 events/matchers, `.cursor/agents/` frontmatter (`model`/`readonly`), and `${env:VAR}` interpolation.

## Install

```bash
# Homebrew (macOS/Linux)
brew install asingamaneni/tap/omniplug

# npm / npx
npm install -g omniplug      # or: npx omniplug --help

# Go (1.23+)
go install github.com/asingamaneni/omniplug/cmd/omniplug@latest
```

Or grab a prebuilt binary from [Releases](https://github.com/asingamaneni/omniplug/releases). Build from source with `make build` (→ `./bin/omniplug`). See [Installation](https://asingamaneni.github.io/omniplug/docs/installation/) for all options.

## Usage

```bash
omniplug init my-plugin                 # scaffold a canonical plugin source (--force to overwrite)
omniplug validate -s my-plugin          # schema checks + the same degradation warnings build prints (no writes)
omniplug build    -s my-plugin -o dist  # compile to dist/<target>/  (-t claude,cursor to select targets)
omniplug install  -s my-plugin --scope project --dry-run   # --project-dir to target another checkout
omniplug list-targets                   # registered adapters + capability matrix
omniplug --version
```

Full command reference, flags, and the canonical frontmatter schema: **[Usage guide →](https://asingamaneni.github.io/omniplug/docs/usage/)**

Claude command output remains backward-compatible by default (`commands/<name>.md`). To compile existing canonical commands as Claude `SKILL.md` files instead, add:

```yaml
targetOptions:
  claude:
    commandEmission: skills
```

The source layout stays `commands/<name>.md`; only the Claude output changes. Converted commands remain slash-invocable and model-invocation-disabled. See the [Usage guide](https://asingamaneni.github.io/omniplug/docs/usage/#claude-command-emission) for compatibility and collision details.

Try it against the bundled example:

```bash
omniplug build -s examples/hello-plugin -o dist
```

## Canonical source layout

```
my-plugin/
├── plugin.yaml              # manifest (single source of truth)
├── skills/<name>/SKILL.md   # portable Agent Skills standard (+ scripts/, references/)
├── commands/<name>.md       # explicit slash-commands / prompts
├── agents/<name>.md         # subagent definitions (body = system prompt)
├── hooks/hooks.yaml         # lifecycle hooks
├── mcp/servers.yaml         # MCP server definitions
└── guidance/AGENTS.md       # shared guidance
```

Frontmatter uses neutral field names and abstract model tiers (`fast | balanced | powerful | inherit`); each adapter maps them to native fields and degrades unsupported ones with a diagnostic. See the design doc for the full mapping tables.

## Project layout

```
cmd/omniplug/        entrypoint (registers adapters via blank import)
internal/model/      canonical IR
internal/parser/     source -> IR
internal/schema/     validation
internal/adapter/    Adapter interface + registry
internal/adapters/   one package per target (claude, cursor, ...)
internal/yamlfm/     shared YAML frontmatter builder
internal/compiler/   orchestration over the registry
internal/installer/  filesystem placement + dry-run
internal/cli/        cobra commands
examples/            sample canonical plugins
```

## Adding a target

1. Create `internal/adapters/<name>/` implementing `adapter.Adapter`.
2. Declare `Capabilities()`, implement `Compile()` (pure: IR → files) and `InstallPlan()`.
3. `func init() { adapter.Register(&Adapter{}) }` and add a blank import in `cmd/omniplug/main.go`.

No edits to the parser, compiler, or CLI.

## Development

```bash
go test ./...
go vet ./...
gofmt -l .
```
