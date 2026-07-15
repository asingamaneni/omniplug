# Security Policy

## Supported versions

Only the latest release receives security fixes.

## Reporting a vulnerability

Please report vulnerabilities privately via
[GitHub Security Advisories](https://github.com/asingamaneni/omniplug/security/advisories/new)
or by email to <ashok.singamaneni90@gmail.com>. Do not open a public issue for security
reports. You can expect an acknowledgement within a few days.

## Scope notes

omniplug parses **untrusted plugin sources** and writes compiled output to
disk, so the following are explicitly in scope:

- Path traversal or writes outside the install root (the installer's
  `safeJoin` zip-slip guard).
- Reading files outside the source tree (the parser refuses symlinks and caps
  file sizes).
- Privilege escalation via emitted file modes (setuid/setgid/sticky bits are
  never propagated).

The npm package's `postinstall` downloads a release binary from GitHub over
HTTPS; issues with that path (URL construction, tampering) are also in scope.
