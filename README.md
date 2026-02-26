<div align="center">

# commitdog

**stop writing commit messages. let commitdog do it.**

reads your staged diff · suggests conventional commits · you pick one · done

[![License: MIT](https://img.shields.io/badge/license-MIT-orange.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey.svg)](#install)
[![No Telemetry](https://img.shields.io/badge/telemetry-none-brightgreen.svg)](#security)
[![part of aysdog](https://img.shields.io/badge/part%20of-aysdog-orange.svg)](https://aysdog.pages.dev)

[![Star History Chart](https://api.star-history.com/svg?repos=aysdog/commitdog&type=Date)](https://star-history.com/#aysdog/commitdog&Date)

</div>

---

## what is this?

You just finished coding. Now you have to write a commit message. You type `"fix stuff"` or `"wip"` or just mash the keyboard. Three weeks later you're reading git log and it's completely useless.

commitdog reads what you actually changed and writes the message for you. You pick one. Done in 5 seconds.

**no AI. no internet needed. no config files. no API keys. single binary.**

---

## install

```sh
curl -fsSL https://get.commitdog.dev | sh
```

that's it. one command. works on Linux and macOS.

<details>
<summary>Windows</summary>

Download `commitdog-windows-amd64.exe` from the [releases page](https://github.com/aysdog/commitdog/releases), rename it to `commitdog.exe` and add it to your PATH.

</details>

<details>
<summary>build from source (needs Go 1.21+)</summary>

```sh
git clone https://github.com/aysdog/commitdog.git
cd commitdog
go build -o commitdog .
sudo mv commitdog /usr/local/bin/commitdog
```

</details>

---

## first time setup (do this once)

```sh
commitdog setup
```

it will ask for:
1. your GitHub noreply email (find it at [github.com/settings/emails](https://github.com/settings/emails))
2. a GitHub personal access token (create one at [github.com/settings/tokens](https://github.com/settings/tokens) — classic token, `repo` + `write:org` scopes)

saved to `~/.config/commitdog/config.toml`. never asked again.

---

## daily use

```sh
# stage your changes like normal
git add .

# run commitdog instead of git commit
commitdog
```

```
  suggestions:

  1  feat(auth): add refreshToken and verifyToken
  2  feat: implement refreshToken in auth
  3  feat: update auth module

  [1/2/3] pick, [e] edit, [q] quit › 1

  ✓ committed: feat(auth): add refreshToken and verifyToken

  push to origin/main? [Y/n] ›
  pushing...
  ✓ pushed to origin/main
```

pick a number. press enter to push. that's the whole thing.

---

## starting a brand new project

no more going to GitHub, creating a repo, copying the URL, setting the remote. commitdog does all of it:

```sh
mkdir my-project
cd my-project
# add your files
commitdog init
```

```
  commitdog init

  ✓ connected as anirbanfaith

  push to personal or org? [P/o] › o
  org name › aysdog
  repo name [my-project] ›
  private or public? [P/u] › u

  ✓ repo created: github.com/aysdog/my-project

  suggestions:
  1  feat: initial project setup
  2  chore: initial commit

  [1/2] pick › 1

  ✓ committed: feat: initial project setup
  ✓ pushed

  live at github.com/aysdog/my-project
```

---

## commands

| command | what it does |
|---------|-------------|
| `commitdog` | suggest commit message for staged changes |
| `commitdog init` | create a GitHub repo and do the first push |
| `commitdog setup` | configure email and GitHub token (do once) |
| `commitdog --version` | print version |
| `commitdog --help` | print help |

---

## how it figures out the message

commitdog parses `git diff --staged` and looks at:

```
what files changed?
  → source, test, config, docs, migration?

what functions were added or removed?
  → add refreshToken, remove oldAuth?

what patterns are in the diff?
  → error handling, logging, test cases?

what's the main folder/module?
  → auth, api, db?
```

then generates 2-3 variations in [conventional commits](https://www.conventionalcommits.org) format and lets you pick.

### commit types it detects

| what changed | type |
|-------------|------|
| new function added | `feat` |
| function removed | `refactor` |
| only tests changed | `test` |
| only docs/README | `docs` |
| error handling added | `fix` |
| config file changed | `chore` |
| dependencies updated | `chore` |
| migration file added | `feat` |
| debug logs removed | `chore` |

### languages supported

Go · JavaScript · TypeScript · Python · Ruby · Rust · Java · Kotlin

---

## security

commitdog is designed to never leak your data.

| concern | how it's handled |
|---------|-----------------|
| shell injection | all git commands use `exec.Command` with explicit args — no shell |
| token storage | saved with `0600` permissions — only you can read it |
| token in git ops | never used — SSH or HTTPS handles push, token only for API |
| diff size | capped at 200KB — no memory issues |
| commit messages | sanitized before passing to git |
| network | zero outbound connections except `commitdog init` (GitHub API) |
| dependencies | pure Go stdlib — no third-party packages |

you can read the entire source in 20 minutes. nothing is hidden.

---

## contributing

open an issue first. if it fits — open a PR.

```
the one rule: don't add telemetry.
everything else is negotiable.
```

---

## part of aysdog

commitdog is part of [aysdog](https://aysdog.pages.dev) — open-source tools for developers who hate bloat.

zero telemetry · self-hostable · single binary · MIT licensed