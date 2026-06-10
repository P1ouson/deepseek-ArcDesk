<p align="center">
  <img src="docs/logo.svg" alt="ArcDesk" width="440"/>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/P1ouson/ArcDesk?style=flat-square&color=8b949e&labelColor=161b22" alt="MIT"/></a>
  <a href="https://github.com/P1ouson/ArcDesk/releases"><img src="https://img.shields.io/github/v/release/P1ouson/ArcDesk?include_prereleases&style=flat-square&color=0153e5&labelColor=161b22" alt="release"/></a>
</p>

<p align="center">
  <strong>English</strong>
  &nbsp;·&nbsp;
  <a href="./README.md">简体中文</a>
  &nbsp;·&nbsp;
  <a href="https://github.com/P1ouson/ArcDesk/releases">Releases</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">Spec</a>
  &nbsp;·&nbsp;
  <a href="./SECURITY.md">Security</a>
  &nbsp;·&nbsp;
  <a href="./CONTRIBUTING.md">Contributing</a>
</p>

<br/>

# ArcDesk

**MIT-licensed, DeepSeek-native coding agent — desktop app + CLI, one Go kernel.**

Long sessions shouldn't pay full price for the entire context every turn. **ArcDesk** is the product name; **`ARCDESK`** is the CLI command and config prefix (`ARCDESK.toml`).

| | |
|---|---|
| **Desktop-first** | **Installers** for Windows, macOS, and Linux on [Releases](https://github.com/P1ouson/ArcDesk/releases) |
| **DeepSeek cost** | Prefix-cache-friendly append-only sessions; optional executor + planner split |
| **Open & controllable** | MCP (stdio + HTTP), `.mcp.json`, TOML permission rules, MIT source |

<p align="center">
  <a href="https://github.com/P1ouson/ArcDesk/releases">
    <img src="docs/screenshots/desktop-workbench.png" alt="ArcDesk desktop workbench" width="900"/>
  </a>
</p>

<p align="center">
  <a href="https://github.com/P1ouson/ArcDesk/releases"><strong>Download desktop</strong></a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="#cli"><strong>Build CLI</strong></a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="./docs/SPEC.md"><strong>Read spec</strong></a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="#faq"><strong>FAQ</strong></a>
</p>

<br/>

## Quick install

### Desktop

| Platform | Installer | Notes |
|----------|-----------|-------|
| **Windows** | [`.exe` setup wizard](https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-windows-amd64-installer.exe) | ~10 MB, pick install folder, no admin |
| **macOS** | [Universal `.dmg`](https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-darwin-universal.dmg) | Drag to Applications; see [Troubleshooting](#troubleshooting) if blocked |
| **Linux** | [`.tar.gz` installer](https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-linux-amd64-installer.tar.gz) | Extract, run `./install.sh` (installs to `~/.local`) |

1. Download the **installer** for your platform from the table or **[GitHub Releases](https://github.com/P1ouson/ArcDesk/releases)** (not the source zip).
2. Open **ArcDesk**, paste your [DeepSeek API key](https://platform.deepseek.com/) on the onboarding screen (stored locally).
3. **Open a project folder** and describe your task.

Linux example:

```sh
tar -xzf arcdesk-desktop-linux-amd64-installer.tar.gz
./install.sh
arcdesk-desktop
```

> First launch: macOS Gatekeeper / Windows SmartScreen may block unsigned builds — see [Troubleshooting](#troubleshooting).

### CLI / build from source {#cli}

**No npm package is published from this repo.** The CLI shares the same Go kernel; build locally:

```sh
make build          # -> bin/ARCDESK(.exe) or bin/arcdesk(.exe)
./bin/arcdesk chat  # interactive terminal
```

Config: [`ARCDESK.example.toml`](./ARCDESK.example.toml) (project) and `~/.config/arcdesk/config.toml` (user).

<br/>

## 60-second start

**Desktop**: install → API key → open project → describe task.

**CLI**

```sh
export DEEPSEEK_API_KEY=sk-...     # or: ARCDESK setup
ARCDESK chat
ARCDESK run "explain this repo"
```

<br/>

## Why ArcDesk?

| | **ArcDesk** | **Cursor** | **Cline / Roo** | **Claude Code** | **OpenCode** |
|---|:---:|:---:|:---:|:---:|:---:|
| **Desktop app** | Native (Wails) | VS Code fork | Editor extension | CLI / plugin | Terminal-first |
| **DeepSeek / cost** | Prefix-cache session design | Multi-model IDE | Model-agnostic | Claude ecosystem | Model-agnostic CLI |
| **MCP** | stdio + HTTP; `.mcp.json` | Ecosystem | Supported | MCP supported | Varies |
| **Local control** | TOML, permissions, sandbox | Account policy | Extension settings | Anthropic account | Config / env |

<br/>

## Security & trust

**Ask before execute** by default. See [`SECURITY.md`](./SECURITY.md) · [`desktop/README.md`](./desktop/README.md) · [`docs/SPEC.md`](./docs/SPEC.md) §9.

<br/>

## FAQ {#faq}

**ArcDesk vs ARCDESK?** — ArcDesk is the product and desktop app; ARCDESK is the CLI command and config namespace. Same kernel underneath.

**Is it free?** — MIT software; model API usage (e.g. DeepSeek) is billed by your provider.

**Must I use the desktop?** — No; `ARCDESK chat` / `run` share the same engine.

**Non-DeepSeek models?** — Any OpenAI-compatible endpoint via `[[providers]]` in `ARCDESK.toml`, but **session design and cost tuning target DeepSeek first**.

**Migrating from 0.x?** — See [`docs/MIGRATING.md`](./docs/MIGRATING.md); legacy on [`v1`](https://github.com/P1ouson/ArcDesk/tree/v1).

<br/>

## Troubleshooting {#troubleshooting}

| Symptom | Fix |
|---------|-----|
| macOS "app is damaged" | Unsigned build: `xattr -dr com.apple.quarantine /Applications/ArcDesk.app` |
| Windows SmartScreen | *More info → Run anyway* |
| Windows blank window | Install [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) |
| Linux blank / flicker | Install WebKitGTK 4.1; try `WEBKIT_DISABLE_COMPOSITING_MODE=1` |
| MCP not loading | Trust project/server in desktop UI; check `.mcp.json` |

More: [GitHub Issues](https://github.com/P1ouson/ArcDesk/issues)

<br/>

---

> **Naming**: **ArcDesk** = product · **ARCDESK** = CLI / config · repo [`P1ouson/ArcDesk`](https://github.com/P1ouson/ArcDesk)

<br/>

## Build from source

```powershell
cd desktop
.\scripts\build-windows-installer.ps1   # needs NSIS
```

```sh
make build
cd desktop && wails build
bash scripts/desktop-build.sh linux/amd64 desktop-v0.1.1   # Linux installer tarball
```

See [`desktop/README.md`](./desktop/README.md).

<br/>

## Lineage — Reasonix

ArcDesk's **Go agent kernel** builds on [**Reasonix**](https://github.com/esengine/DeepSeek-Reasonix). Thank you to the Reasonix project and contributors.

This repo adds desktop-first work: **Wails studio UI**, **three-platform installers**, **security hardening**, and **ArcDesk branding** (`arcdesk.toml`, non-destructive import from `~/.reasonix/`).

Details: [`SECURITY.md`](./SECURITY.md) · [`docs/MIGRATING.md`](./docs/MIGRATING.md).

<br/>

## Acknowledgments

The Go agent kernel references [**Reasonix**](https://github.com/esengine/DeepSeek-Reasonix) and its contributors (see [Lineage](#lineage--reasonix) above).

<br/>

---

<p align="center">
  <sub>MIT — see <a href="./LICENSE">LICENSE</a></sub>
  <br/>
  <sub><a href="https://github.com/P1ouson/ArcDesk">P1ouson/ArcDesk</a></sub>
</p>
