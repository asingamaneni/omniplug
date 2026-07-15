# omniplug (npm distribution)

This package is a thin wrapper that downloads the prebuilt `omniplug` binary for
your platform from the project's GitHub Releases.

```bash
npm install -g omniplug
# or run without installing:
npx omniplug --help
```

The real project lives at https://github.com/asingamaneni/omniplug.

Publishing is automated by `.github/workflows/release.yml`: on a `v*` tag, the
package `version` is set from the tag and published to npm (requires the
`NPM_TOKEN` repo secret). The version must match a GitHub release that GoReleaser
produced for the same tag, so the binary assets exist.
