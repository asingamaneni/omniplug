# omniplug

Author an AI agent plugin **once** in a tool-neutral canonical format, then compile or install it into target-specific layouts. Claude Code and Cursor are supported today; Codex and future tools (Grok, Gemini CLI, …) slot in by implementing one adapter — no changes to the core.

Docs: **https://asingamaneni.github.io/omniplug** · Design: [`site/content/docs/architecture.md`](site/content/docs/architecture.md)

## Status

End-to-end pipeline (parse → IR → validate → compile → install) with two target adapters:

| Target | Skills | MCP | Commands | Agents | Hooks | Guidance |
| ------ | :----: | :-: | :------: | :----: | :---: | :------: |
| **claude** | yes | yes | native | yes | yes | yes |
| **cursor** | yes | yes | rules | yes | yes | yes |

Both targets support every component natively. Where a canonical field has no native home (e.g. an agent's explicit tool allowlist or a fine model tier on Cursor), the adapter degrades it with a diagnostic instead of producing incorrect output. **Codex** is next. Adding it requires only a new adapter package — no changes to the parser, compiler, or CLI.

Output formats were validated against the official Claude Code and Cursor documentation (plugin `hooks.json` wrapping, `.mcp.json` remote-server shape, Cursor `.cursor/hooks.json` + `.cursor/agents/` native formats).

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
omniplug init my-plugin                 # scaffold a canonical plugin source
omniplug validate -s my-plugin          # schema + per-adapter checks (no writes)
omniplug build    -s my-plugin -o dist  # compile to dist/<target>/
omniplug install  -s my-plugin --scope project --dry-run
omniplug list-targets                   # registered adapters + capability matrix
```

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
