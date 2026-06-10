# Security Policy

ArcDesk runs tools on your machine (shell, file writes, MCP servers). Treat it like
any agent that can modify code and execute commands.

## Supported versions

| Version | Supported |
|---------|-----------|
| **1.x** (Go rewrite, `main-v2`) | Yes — security fixes for the current release line |
| **0.x** (TypeScript, `v1` branch) | Maintenance only — prefer upgrading to 1.x |

Desktop builds are tagged `desktop-v*`. CLI/npm releases follow semver on the 1.x line.

## Reporting a vulnerability

**Do not open a public GitHub issue for security bugs.**

1. Email maintainers privately (see GitHub profile contact or open a
   [Security Advisory](https://github.com/esengine/DeepSeek-ARCDESK/security/advisories/new)
   on the upstream repository if you have access).
2. Include: affected version, platform (desktop / CLI / OS), reproduction steps, and
   impact (local privilege, network exposure, secret leak, etc.).
3. We aim to acknowledge within **72 hours** and share a fix timeline when confirmed.

Reasonable disclosure: please allow time for a patch and release before public
details. We credit reporters in the advisory when they want attribution.

## Security model (summary)

| Layer | Behavior |
|-------|----------|
| **Secrets** | API keys via environment or desktop credential store — not in `ARCDESK.toml` |
| **Permissions** | Per-tool `allow` / `ask` / `deny`; `deny` always wins |
| **Sandbox** | File writers confined to workspace root; macOS `bash` jailed by default (Seatbelt) |
| **MCP trust** | Repo `.mcp.json` servers quarantined until explicitly trusted per project |
| **Native confirms** | Sensitive UI actions (credentials, tunnels, LAN, high-risk shell) require OS dialogs |
| **Updates** | Windows/Linux verify **minisign + SHA256** before apply ([`desktop/README.md`](./desktop/README.md)) |

Full contract: [`docs/SPEC.md`](./docs/SPEC.md) (permissions, sandbox §9).

## Verifying downloads

Release artifacts are signed with **minisign** (public key ID `AF12CA46F4A9EBB0`):

```sh
minisign -Vm ARCDESK-linux-amd64.tar.gz \
  -P RWSw66n0RsoSr6Zhh6qt5YO95YkpCayTOCMFVDNUQSjJYwxoYngNVBSq
```

Checksums and `.minisig` files ship alongside each asset on
[GitHub Releases](https://github.com/esengine/DeepSeek-ARCDESK/releases).

## Known limitations

- **macOS / Windows desktop builds** are not yet notarized / Authenticode-signed;
  first launch requires explicit user override (see [`desktop/README.md`](./desktop/README.md)).
- **`bash` sandbox** is strongest on macOS; Linux bubblewrap and Windows confinement
  are still evolving — see `docs/SPEC.md` §9.
- **Mobile / relay / tunnel features** expose additional network surfaces; review
  settings before enabling LAN or public tunnels.

## Safe defaults for operators

- Keep `[permissions].mode = "ask"` until you trust a workflow.
- Trust MCP servers per project; do not blanket-trust unknown `.mcp.json` sources.
- Run desktop updates only from official release URLs or the in-app updater CDN mirror.
- Do not commit API keys, `trust.json`, or channel config files to version control.
