---
title: omniplug
type: docs
---

# omniplug

Author an AI agent plugin **once** in a tool-neutral canonical format, then compile or install it into target-specific layouts. **Claude Code** and **Cursor** are supported today; Codex and future tools (Grok, Gemini CLI, …) slot in by implementing one adapter — no changes to the core.

```bash
omniplug init my-plugin                 # scaffold a canonical plugin source
omniplug validate -s my-plugin          # schema + per-adapter checks (no writes)
omniplug build    -s my-plugin -o dist  # compile to dist/<target>/
omniplug install  -s my-plugin --scope project --dry-run
omniplug list-targets                   # registered adapters + capability matrix
```

## Why

Every AI coding agent exposes the same conceptual extension points — skills, MCP servers, slash commands, subagents, lifecycle hooks — but each wraps them in a different on-disk layout and manifest. Maintaining N parallel copies drifts over time. omniplug keeps **one source of truth** and compiles correct, per-target output.

## Target support

| Target | Skills | MCP | Commands | Agents | Hooks | Guidance |
| ------ | :----: | :-: | :------: | :----: | :---: | :------: |
| **claude** | yes | yes | native | yes | yes | yes |
| **cursor** | yes | yes | rules | yes | yes | yes |

Where a canonical field has no native home on a target, the adapter degrades it with a diagnostic rather than emitting incorrect output. Output formats are validated against the official Claude Code and Cursor docs.

Get started with **[Installation](docs/installation.md)**, then **[Usage](docs/usage.md)**.
