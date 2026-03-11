<div align="center">

# commitdog

**stop writing commit messages. let commitdog do it.**

reads your diff · stages your files · suggests conventional commits · you pick one · done

[![License: MIT](https://img.shields.io/badge/license-MIT-orange.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey.svg)](#install)
[![No Telemetry](https://img.shields.io/badge/telemetry-none-brightgreen.svg)](#security)
[![part of aysdog](https://img.shields.io/badge/part%20of-aysdog-orange.svg)](https://aysdog.com)

[![Star History Chart](https://api.star-history.com/svg?repos=aysdog/commitdog&type=Date)](https://star-history.com/#aysdog/commitdog&Date)

</div>

---

## what is this?

You just finished coding. Now you have to write a commit message. You type `"fix stuff"` or `"wip"` or just mash the keyboard. Three weeks later you're reading git log and it's completely useless.

commitdog reads what you actually changed and writes the message for you. You pick one. Done in 5 seconds.

**no AI. no internet needed. no config files. no API keys. single binary.**

---

## install

**Linux and macOS**

```sh
curl -fsSL https://aysdog.com/install-commitdog.sh | sh
```

**macOS (Homebrew)**

```sh
brew tap aysdog/commitdog
brew install commitdog
```

**Arch Linux (AUR)**

```sh
yay -S commitdog-bin
```

**Windows** — open PowerShell as Administrator and run:

```powershell
irm https://aysdog.com/install-commitdog.ps1 | iex
```

**Windows (winget)**

```sh
winget install aysdog.commitdog
```

downloads the binary, adds it to PATH automatically. restart your terminal and `commitdog` just works.

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
# just run commitdog — it stages everything and suggests a message
commitdog

# or stage a specific file only
commitdog auth.go
```

```
  nothing staged. staging all changes...

  suggestions:

  1  feat(auth): add refreshToken and verifyToken
  2  feat: implement refreshToken in auth
  3  feat: update auth module
  4  feat(auth): add refreshToken, add verifyToken — update middleware

  [1/2/3/4] pick, [e] edit, [q] quit › 1

  ✓ committed: feat(auth): add refreshToken and verifyToken

  push to origin/main? [Y/n] ›
  pushing...
  ✓ pushed to origin/main
```

pick a number. press enter to push. that's the whole thing.

---

## branch, sync, stash, merge

```sh
commitdog branch    # branch menu — switch, create, delete
commitdog switch    # jump straight to branch switcher
commitdog merge     # merge a branch into current with preview
commitdog sync      # fetch + rebase + push in one shot
commitdog stash     # save work in progress
commitdog --update  # update commitdog itself
```

---

## git log with branch graph

```sh
commitdog log
```

```
  main
  │
  ● e3e28ab (HEAD -> main)  Revert "Merge branch 'feat_3'"
  │
  ● 160bf5a  Merge branch 'feat_3'
  ├─╮
  │ ● 1d85daf (feat_3)  feat: add new feature
  │ 
  ●     831ee73  feat: add a, add b, add c
  │
```

each branch gets its own color. `j`/`k` scroll · `a` show all · `q` quit

---

## pull requests

```sh
commitdog pr
```

on a feature branch — opens an interactive diff viewer then creates the PR. on main — lists open PRs, lets you review the diff, merge, or close.

```
  creating PR: feat_3 → main

  ┌─ config.go           +23 -4  ██████████░░░░░░
  └─ README.md           +5  -2  █████░░░░░░░░░░░

  2 files changed  +28 -6

  [↑/↓] navigate  [enter] view diff  [c] create PR  [q] quit
```

merge strategies: merge commit · squash · rebase. deletes the remote branch after merge.

---

## release

```sh
commitdog release
```

```
  detected: Go  ·  current version: v0.2.1

  1  patch  →  v0.2.2
  2  minor  →  v0.3.0
  3  major  →  v1.0.0
  4  custom

  [1/2/3/4/q] pick › 1

  bumping version in main.go...              ✓
  building linux/amd64...                    ✓
  building linux/arm64...                    ✓
  building darwin/amd64...                   ✓
  building darwin/arm64...                   ✓
  building windows/amd64...                  ✓
  committing...                              ✓
  tagging v0.2.2...                          ✓
  pushing...                                 ✓
  creating GitHub release...                 ✓
  uploading commitdog-linux-amd64...         ✓
  uploading commitdog-linux-arm64...         ✓
  uploading commitdog-darwin-amd64...        ✓
  uploading commitdog-darwin-arm64...        ✓
  uploading commitdog-windows-amd64.exe...   ✓

  ✓ v0.2.2 released
  https://github.com/aysdog/commitdog/releases/tag/v0.2.2
```

auto-detects Go · Node.js · Rust · Python · Java. if no version file exists, offers to create one.

---

## made a mistake? revert it

```sh
commitdog revert
```

handles merge commits too — shows a branch picker instead of crashing.

```
  recent commits:

  1  63baabe  Merge branch 'feat_3'         (2 minutes ago)
  2  3b7486d  feat(auth): add refreshToken   (1 hour ago)
  3  c90ace2  refactor: update 10 files      (6 hours ago)

  [1-3] pick, [e] enter hash, [q] quit › 1

  this is a merge commit. which side to revert to?

  1  main    (undo the merge entirely)
  2  feat_3  (the branch that was merged in)

  [1/2] › 1

  ✓ reverted 63baabe
  ✓ pushed to origin/main
```

---

## starting a brand new project

```sh
mkdir my-project && cd my-project
commitdog init
```

creates the GitHub repo, git init, first commit, first push — no browser needed.

---

## commands

| command | what it does |
|---------|-------------|
| `commitdog` | stage all, suggest message, commit, push |
| `commitdog <file>` | stage specific file, suggest, commit, push |
| `commitdog log` | interactive git log with colored branch graph |
| `commitdog pr` | create PR from feature branch · list/review/merge from main |
| `commitdog release` | bump version, build 5 binaries, tag, push, GitHub release |
| `commitdog branch` | branch menu — switch / create / delete |
| `commitdog switch` | jump straight to branch switcher |
| `commitdog branch create` | create new branch, names suggested from diff |
| `commitdog branch ls` | list all local and remote branches |
| `commitdog branch delete` | delete branch locally + optionally remote |
| `commitdog merge` | merge with diff preview, conflict detection |
| `commitdog sync` | fetch + pull rebase + push |
| `commitdog stash` | save, pop, or drop stashes interactively |
| `commitdog revert` | pick from last 5 commits, revert, handles merge commits |
| `commitdog init` | create GitHub repo, first commit, first push |
| `commitdog setup` | configure email and GitHub token (once) |
| `commitdog --update` | update to latest release |
| `commitdog --version` | print version with ascii art |
| `commitdog --help` | print help |

---

## how it figures out the message

commitdog parses `git diff --staged` and looks at what files changed, what functions were added or removed, what the branch name suggests, and what module or folder is affected. then generates 4 variations in [conventional commits](https://www.conventionalcommits.org) format.

### languages supported

Go · JavaScript · TypeScript · Python · Ruby · Rust · Java · Kotlin

---

## security

| concern | how it's handled |
|---------|-----------------|
| shell injection | all git commands use `exec.Command` with explicit args — no shell |
| token storage | saved with `0600` permissions — only you can read it |
| token in git ops | never used — SSH or HTTPS handles push, token only for GitHub API |
| diff size | capped at 200KB |
| network | zero outbound except `init`/`pr`/`release` GitHub API calls and your git remote |
| dependencies | pure Go stdlib — zero third-party packages |

you can read the entire source in 20 minutes. nothing is hidden.

---

## version history

| version | what shipped |
|---------|-------------|
| v0.2.1 | Homebrew · AUR · winget · smart version init for unknown project types |
| v0.2.0 | `commitdog pr` · `commitdog release` · interactive diff viewer · log graph fixes |
| v0.1.9 | HTTPS remote auto-fix to SSH · merge commit revert with branch picker |
| v0.1.8 | `commitdog log` with colored branch graph · fastfetch-style `--version` |
| v0.1.4 | branch · sync · stash · self-update · ascii art |
| v0.1.2 | revert · Windows installer |
| v0.1.0 | initial release |

---

## contributing

open an issue first. if it fits — open a PR.

```
the one rule: don't add telemetry.
everything else is negotiable.
```

---

## part of aysdog

commitdog is part of [aysdog](https://aysdog.com) — open-source tools for developers who hate bloat.

zero telemetry · self-hostable · single binary · MIT licensed