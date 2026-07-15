---
title: Usage
weight: 20
---

# Usage

## Commands

| Command | What it does |
| ------- | ------------ |
| `omniplug init [name]` | Scaffold a new canonical plugin source. |
| `omniplug validate -s <src>` | Schema + per-adapter checks. No files written. |
| `omniplug build -s <src> -o <dir> [-t all\|claude\|cursor]` | Compile to `<dir>/<target>/`. |
| `omniplug install -s <src> --scope project\|user [--dry-run]` | Compile and place files in the target's real config location. |
| `omniplug list-targets` | Show registered adapters and their capability matrix. |

`--dry-run` and `validate` never touch disk, so they're safe in CI and pre-commit hooks.

## Canonical source layout

```text
my-plugin/
├── plugin.yaml              # manifest (single source of truth)
├── skills/<name>/SKILL.md   # portable Agent Skills standard (+ scripts/, references/)
├── commands/<name>.md       # explicit slash-commands / prompts
├── agents/<name>.md         # subagent definitions (body = system prompt)
├── hooks/hooks.yaml         # lifecycle hooks (+ hooks/scripts/ are bundled)
├── mcp/servers.yaml         # MCP server definitions
└── guidance/AGENTS.md       # shared guidance
```

## Manifest (`plugin.yaml`)

```yaml
apiVersion: omniplug/v1
name: my-plugin
version: 0.1.0
description: One-line summary used for discovery.
author:
  name: Your Name
  url: https://github.com/you/my-plugin
license: MIT
```

## Frontmatter

Component frontmatter uses neutral field names and **abstract model tiers** — `fast`, `balanced`, `powerful`, or `inherit` — and each adapter maps them to native fields, degrading unsupported ones with a diagnostic. For example, a skill:

```yaml
---
name: summarize-changes
description: Summarize uncommitted changes and flag risks.
whenToUse: When the user asks what changed.
allowedTools: [Read, Grep]
model: balanced
---

Summarize the current changes in two or three bullet points...
```

Environment references in MCP configs are written in the standard `${VAR}` form; the Cursor adapter rewrites them to Cursor's required `${env:VAR}` automatically.

See the full per-field mapping tables in **[Architecture](architecture.md#component-metadata-frontmatter-schemas)**.

## Example

A complete example exercising every component lives in [`examples/hello-plugin`](https://github.com/asingamaneni/omniplug/tree/main/examples/hello-plugin):

```bash
omniplug build -s examples/hello-plugin -o dist
# -> dist/claude/  and  dist/cursor/
```
