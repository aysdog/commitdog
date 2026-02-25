<div align="center">

# commitdog

**zero-bs commit message generator**

reads your staged diff · suggests conventional commits · you pick one · done

[![License: MIT](https://img.shields.io/badge/license-MIT-orange.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey.svg)](#install)
[![No Telemetry](https://img.shields.io/badge/telemetry-none-brightgreen.svg)](#security)
[![part of aysdog](https://img.shields.io/badge/part%20of-aysdog-orange.svg)](https://aysdog.pages.dev)

</div>

---

## what it does

```
$ git add .
$ commitdog

  suggestions:

  1  feat(auth): add refreshToken and verifyToken
  2  feat: implement refreshToken in auth
  3  feat: add 2 new functions to auth module

  [1/2/3] pick, [e] edit, [q] quit › 1

✓ committed: feat(auth): add refreshToken and verifyToken

  push to origin/main? [y/n] › y
  ✓ pushed to origin/main
```

no AI. no API keys. no config files. no network calls. just works.

---

## install

<details>
<summary><strong>macOS (Apple Silicon)</strong></summary>

```bash
curl -sL https://github.com/aysdog/commitdog/releases/latest/download/commitdog-darwin-arm64 -o commitdog
chmod +x commitdog && sudo mv commitdog /usr/local/bin/commitdog
```
</details>

<details>
<summary><strong>macOS (Intel)</strong></summary>

```bash
curl -sL https://github.com/aysdog/commitdog/releases/latest/download/commitdog-darwin-amd64 -o commitdog
chmod +x commitdog && sudo mv commitdog /usr/local/bin/commitdog
```
</details>

<details>
<summary><strong>Linux (amd64)</strong></summary>

```bash
curl -sL https://github.com/aysdog/commitdog/releases/latest/download/commitdog-linux-amd64 -o commitdog
chmod +x commitdog && sudo mv commitdog /usr/local/bin/commitdog
```
</details>

<details>
<summary><strong>Linux (arm64)</strong></summary>

```bash
curl -sL https://github.com/aysdog/commitdog/releases/latest/download/commitdog-linux-arm64 -o commitdog
chmod +x commitdog && sudo mv commitdog /usr/local/bin/commitdog
```
</details>

<details>
<summary><strong>Windows (amd64)</strong></summary>

Download `commitdog-windows-amd64.exe` from the [releases page](https://github.com/aysdog/commitdog/releases), rename it to `commitdog.exe` and add it to your PATH.

</details>

<details>
<summary><strong>Build from source</strong></summary>

```bash
git clone https://github.com/aysdog/commitdog.git
cd commitdog
go build -o commitdog .
sudo mv commitdog /usr/local/bin/commitdog
```

requires Go 1.21+

</details>

---

## usage

```bash
# stage your changes
git add .

# run commitdog
commitdog
```

| key | action |
|-----|--------|
| `1` `2` `3` | pick a suggestion |
| `e` | edit the message before committing |
| `q` | quit without committing |
| `y` / `n` | push or skip after commit |

---

## how it works

commitdog parses `git diff --staged` directly — no temp files, no shell pipes, no network.

```
git diff --staged
      │
      ▼
  parse files
  ┌─────────────────────────────────┐
  │  which files changed            │
  │  added / removed functions      │
  │  patterns detected              │
  │  scope from folder structure    │
  └─────────────────────────────────┘
      │
      ▼
  infer type + scope
  ┌─────────────────────────────────┐
  │  feat / fix / refactor /        │
  │  docs / test / chore / style    │
  └─────────────────────────────────┘
      │
      ▼
  generate 2-3 suggestions
      │
      ▼
  you pick → git commit -m "..."
```

### what it detects

| change | commit type |
|--------|------------|
| new function added | `feat` |
| function removed | `refactor` |
| only test files changed | `test` |
| only docs / README changed | `docs` |
| error handling added | `fix` |
| config file changed | `chore` |
| dependency file changed | `chore` |
| migration file added | `feat` |
| debug logs removed | `chore` |

### language support

| language | function detection |
|----------|--------------------|
| Go | `func`, methods |
| JavaScript / TypeScript | `function`, arrow functions |
| Python | `def`, `class` |
| Ruby | `def` |
| Rust | `fn` |
| Java / Kotlin | method declarations |

---

## security

commitdog is designed to never leak data.

| concern | how it's handled |
|---------|-----------------|
| shell injection | all git commands use `exec.Command` with explicit args — no shell |
| memory safety | staged diff capped at 200KB |
| commit message | sanitized before passing to git — null bytes and metacharacters stripped |
| push safety | branch and remote names validated against allowlist before use |
| network | zero outbound connections — ever |
| dependencies | pure Go stdlib — no third-party packages |

you can read the entire source in 20 minutes. there is nothing hidden.

---

## flags

```
commitdog            run normally
commitdog --version  print version
commitdog --help     print help
```

---

## contributing

open an issue describing what you want to change. if it fits the philosophy — open a PR.

```
the one rule: don't add telemetry.
everything else is negotiable.
```

fork it, rename it, ship your own version. MIT licensed. we'll probably star the repo.

---

## part of aysdog

commitdog is part of [aysdog](https://aysdog.pages.dev) — a collection of open-source tools built for developers who hate bloat.

zero telemetry · self-hostable · single binary · MIT licensed