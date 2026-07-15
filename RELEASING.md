# Releasing omniplug

## One-time setup (before the first release)

1. **Homebrew tap repo** — create `github.com/asingamaneni/homebrew-tap`
   (public, can be empty). GoReleaser pushes the generated formula there; it
   does **not** create the repo, and the release job fails without it.
2. **Repo secrets** (Settings → Secrets and variables → Actions):
   - `HOMEBREW_TAP_TOKEN` — a fine-grained PAT with `contents: write` on
     `homebrew-tap` (the default `GITHUB_TOKEN` cannot push to another repo).
   - `NPM_TOKEN` — an npm automation token that can publish `omniplug`.
3. **GitHub Pages** — `docs.yml` auto-enables Pages (Actions source) via
   `configure-pages` on its first run. If your org restricts that, set it
   manually: Settings → Pages → Source: **GitHub Actions**. (The `ci.yml`
   build/test/lint jobs are independent and pass regardless.)

## Cutting a release

```bash
make release-check          # validate .goreleaser.yaml
git tag vX.Y.Z && git push origin vX.Y.Z
```

The tag triggers `.github/workflows/release.yml`:

1. **goreleaser** builds linux/darwin/windows × amd64/arm64, publishes the
   GitHub release with checksums, and pushes the Homebrew formula to the tap.
2. **npm** sets `npm/package.json`'s version from the tag
   (`npm version ${TAG#v}` — the committed `0.0.0` is a placeholder by
   design) and publishes. The postinstall script downloads the matching
   `omniplug_<os>_<arch>` release asset, so npm publish must run **after**
   goreleaser — the workflow already orders this with `needs:`.

Update `CHANGELOG.md` before tagging; goreleaser's generated notes exclude
`docs:`/`test:`/`chore:` commits.

## Smoke test after releasing

```bash
brew install asingamaneni/tap/omniplug && omniplug --version
npx -y omniplug@latest --version
go install github.com/asingamaneni/omniplug/cmd/omniplug@latest
```

## Known deprecations

- GoReleaser's `brews:` block is deprecated in favor of `homebrew_casks`
  (removal planned for GoReleaser v3; the workflow pins `~> v2`, which keeps
  `brews` working). Migrating to casks requires code signing/notarization or
  `no_quarantine`, since cask-installed unsigned binaries get Gatekeeper
  quarantine on macOS — revisit when moving to v3.
