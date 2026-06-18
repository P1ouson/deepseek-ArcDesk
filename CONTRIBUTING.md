# Contributing to ArcDesk

Thank you for contributing. **ArcDesk** is the product; **`ARCDESK`** is the CLI binary and config namespace. Repository: [`P1ouson/deepseek-ArcDesk`](https://github.com/P1ouson/deepseek-ArcDesk).

## Security issues

**Do not open public issues for vulnerabilities.** See [`SECURITY.md`](./SECURITY.md).

## Prerequisites

- **Go 1.25+** (see `go.mod` / `toolchain` pin — run `make check-toolchain`)
- **Git**
- **Node.js + pnpm** — only for `desktop/` frontend work

## Getting started

```bash
git clone https://github.com/P1ouson/deepseek-ArcDesk.git
cd deepseek-ArcDesk
make build          # bin/ARCDESK
make test           # full CLI test suite
```

Desktop development:

```bash
cd desktop
wails dev           # hot-reload Go + React
```

See [`desktop/README.md`](./desktop/README.md) for platform webview prerequisites.

## Project structure

| Directory | Purpose |
|-----------|---------|
| `cmd/arcdesk` | CLI entry point |
| `internal/boot` | Assembles `control.Controller` for all clients |
| `internal/agent` | Agent loop, session, coordinator |
| `internal/cli` | TUI, subcommands, setup wizard |
| `internal/control` | Transport-agnostic controller |
| `internal/config` | TOML configuration loading |
| `internal/tool/builtin` | Built-in tools (bash, read_file, …) |
| `internal/provider` | Model-backend abstraction |
| `internal/plugin` | MCP client (stdio + HTTP) |
| `internal/skill` | Skills and sub-agents |
| `internal/serve` | HTTP/SSE server |
| `internal/hook` | Project hooks |
| `internal/knowledge` | Knowledge Studio / failure memory |
| `desktop/` | Wails desktop app (separate Go module) |
| `docs/` | Engineering spec, migration guide |

Dependency direction: `cli → {agent, plugin, config} → {tool, provider}`.

## Development workflow

```bash
make build          # CLI + plugin example
make test           # go test ./...
make vet            # go vet ./...
make fmt            # gofmt -w .
make hooks          # install git hooks (pre-push: go vet)
make cross          # cross-compile six CLI targets
make check-toolchain
```

Desktop tests:

```bash
cd desktop && go test ./...
cd desktop/frontend && pnpm exec tsc --noEmit
```

### Code style

- `gofmt` before commit
- Wrap errors: `fmt.Errorf("...: %w", err)`
- Library code never calls `os.Exit` or prints to stdout/stderr
- Only `cli/` and `main/` decide exit codes and user-facing messages
- Exported identifiers need doc comments

### Commit messages

[Conventional Commits](https://www.conventionalcommits.org/):

```
feat(glob): add ** recursive pattern support
fix: replace silent error discards with structured logging
docs: polish README for desktop-first launch
ci: add go test workflow
```

## Adding a built-in tool

1. Create `internal/tool/builtin/mytool.go`
2. Implement `tool.Tool`: `Name()`, `Description()`, `Schema()`, `ReadOnly()`, `Execute()`
3. Register: `func init() { tool.RegisterBuiltin(myTool{}) }`
4. Add tests

## Adding a model provider

1. Create `internal/provider/myprovider/`
2. Implement `provider.Provider`: `Name()`, `Stream()`
3. Register: `func init() { provider.Register("mykind", New) }`

## Adding i18n strings

1. Field in `internal/i18n/i18n.go` (`Messages` struct)
2. Values in `messages_en.go` and `messages_zh.go`
3. `TestCatalogsComplete` must pass

## Submitting changes

1. Fork [`P1ouson/deepseek-ArcDesk`](https://github.com/P1ouson/deepseek-ArcDesk)
2. Branch from **`main`** (default development branch)
3. Include tests where behavior changes
4. `make test` and `make vet` pass
5. Open a PR to **`main`**

## Releases (maintainers)

| Artifact | Tag / channel |
|----------|----------------|
| CLI | semver tags on `main` |
| Desktop | `desktop-v*` tags → signed builds, `latest.json`, GitHub Releases + CDN mirror |

Signing: minisign (see [`desktop/README.md`](./desktop/README.md)). Do not publish unsigned artifacts outside the release pipeline.

## Reporting bugs

Use [issue templates](https://github.com/P1ouson/deepseek-ArcDesk/issues/new/choose). Include:

- ArcDesk desktop version **or** `ARCDESK --version`
- OS and architecture
- Steps to reproduce, expected vs actual
- Redacted logs (no API keys)

Usage questions → [Discussions](https://github.com/P1ouson/deepseek-ArcDesk/discussions) or [Discord](https://discord.gg/XF78rEME2D).

## License

Contributions are licensed under the project **MIT** license ([`LICENSE`](./LICENSE)).
