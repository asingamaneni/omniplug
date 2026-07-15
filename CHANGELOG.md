# Changelog

All notable changes to omniplug are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] — 2026-07-15

First public release.

### Added

- Canonical, tool-neutral plugin format: `plugin.yaml` manifest plus
  `skills/`, `commands/`, `agents/`, `hooks/hooks.yaml`, `mcp/servers.yaml`,
  and `guidance/AGENTS.md`, with abstract model tiers
  (`fast|balanced|powerful|inherit`) and a per-target `targets:` escape hatch.
- End-to-end pipeline: parse → IR → validate → compile → install, with an
  adapter registry so new targets need one package and zero core changes.
- **Claude Code adapter**: `.claude-plugin/plugin.json` (author, license,
  homepage, repository, keywords), skills/commands/agents, `hooks/hooks.json`
  with `${CLAUDE_PLUGIN_ROOT}` rewriting for bundled scripts, `.mcp.json`,
  `CLAUDE.md`.
- **Cursor adapter**: `.cursor/` skills, rules (commands → Agent-Requested
  `.mdc`), native subagents (`model`, derived `readonly`), `hooks.json` v1
  with the full canonical event map and Claude→Cursor matcher translation
  (`Bash`→`Shell`, edit-family→`Write`), `mcp.json` with `${env:VAR}`
  interpolation.
- Graceful degradation everywhere: any canonical field a target cannot
  express is dropped **with a diagnostic**, never silently.
- CLI: `init` (overwrite-safe, `--force`), `validate` (dry compile — prints
  the same warnings `build` does, writes nothing), `build`, `install`
  (`--scope project|user`, `--project-dir`, `--dry-run`), `list-targets`.
- Security posture: symlink refusal and 10 MiB per-file cap in the parser,
  zip-slip guard in the installer, setuid/setgid/sticky bits never propagated.
- Distribution: Homebrew tap, npm wrapper (`npm i -g omniplug`), prebuilt
  binaries via GoReleaser, `go install`.

[Unreleased]: https://github.com/asingamaneni/omniplug/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/asingamaneni/omniplug/releases/tag/v0.1.0
