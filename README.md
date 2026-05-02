<div align="center">

# commitdog

![license](https://img.shields.io/badge/license-MIT-brightgreen?style=flat-square) ![go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go&logoColor=white) ![platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey?style=flat-square) ![telemetry](https://img.shields.io/badge/telemetry-none-4ade80?style=flat-square) ![part of](https://img.shields.io/badge/part%20of-aysdog-orange?style=flat-square)

git workflow CLI. zero dependencies. single binary.

commit messages, branch management, sync, log, PR, release — all from one tool. pure Go stdlib. no AI. no telemetry.

---

[![Star History Chart](https://api.star-history.com/svg?repos=aysdog/commitdog&type=Date)](https://star-history.com/#aysdog/commitdog&Date)

</div>

---

## install

**Linux / macOS**
```sh
curl -fsSL https://aysdog.com/install-commitdog.sh | sh
```

**macOS (Homebrew)**
```sh
brew tap aysdog/commitdog && brew install commitdog
```

**Arch Linux (AUR)**
```sh
yay -S commitdog-bin
```

**Windows (PowerShell)**
```powershell
irm https://aysdog.com/install-commitdog.ps1 | iex
```

---

## setup

run once to save your GitHub email and a classic PAT:

```sh
commitdog setup
```

stored at `~/.config/commitdog/config.toml` with `0600` permissions. never leaves your machine except when calling the GitHub API.

you need a **classic** Personal Access Token with `repo`, `write:org`, and `read:user` scopes. get one at [github.com/settings/tokens](https://github.com/settings/tokens).

---

## commands

| command | what it does |
|---------|-------------|
| `commitdog` | stage all, suggest 4 commit messages, pick, commit, push |
| `commitdog <file>` | stage a specific file only |
| `commitdog init` | create GitHub repo, git init, first commit, first push |
| `commitdog setup` | save GitHub email and PAT |
| `commitdog log` | interactive git log with colored branch graph |
| `commitdog branch` | interactive branch menu |
| `commitdog switch` | jump to branch switcher |
| `commitdog branch create` | create new branch with optional base |
| `commitdog branch delete` | delete branch locally + optionally remote |
| `commitdog merge` | merge a branch into current with diff preview |
| `commitdog pr` | create PR on feature branch · list/review/merge PRs on main |
| `commitdog sync` | fetch + pull rebase + push — auto-recovers on errors |
| `commitdog stash` | save, pop, or drop stashes interactively |
| `commitdog revert` | pick from last 5 commits and revert |
| `commitdog release` | bump version, build 5 binaries, changelog, tag, push, GitHub release + checksums |
| `commitdog release config` | configure which platforms to build for |
| `commitdog release --changelog-only` | preview grouped changelog since last tag |
| `commitdog status` | project dashboard — commits, PRs, branches, version |
| `commitdog secrets` | scan full commit history for leaked secrets |
| `commitdog --update` | update to latest release |
| `commitdog --version` | print version |

---

## commit

```sh
commitdog
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
  ✓ pushed to origin/main
```

suggestions come from your actual diff — function names, file types, scope inference. no AI, no network call. pick `e` to edit before committing.

---

## secret detection

before showing suggestions, commitdog silently scans your staged diff. if it finds a secret it blocks and shows exactly where:

```
  ✗ possible secret detected in staged changes:

  · AWS access key  in config.go
    var awsKey = "AKIAIOSFODNN7EXAMPLE"

  commit anyway? this will push secrets to your remote. [y/N] ›
```

catches AWS keys, GitHub tokens, private keys, Stripe keys, Slack tokens, generic passwords and API keys. skips `_test` files automatically.

---

## sync with auto-recovery

```sh
commitdog sync
```

if push fails with HTTPS auth:

```
  push failed: GitHub no longer supports HTTPS password auth.
  switch remote to SSH? (git@github.com:you/repo.git) [Y/n] › Y
  ✓ remote switched to SSH
  retrying push...
  ✓ pushed to origin/main
```

other auto-recoveries: non-fast-forward, stale remote tag, missing upstream, protected branch.

---

## release with atomic rollback

```sh
commitdog release
```

```
  detected: Go  ·  current version: v0.2.6

  1  patch  →  v0.2.7
  2  minor  →  v0.3.0
  3  major  →  v1.0.0
  4  custom

  [1/2/3/4/q] pick › 1

  changelog preview:
  ### Bug Fixes
  - fix(sync): handle missing upstream on first push

  release v0.2.6 → v0.2.7? [y/n] › y

  bumping version in main.go...              ✓
  building linux/amd64...                    ✓
  building linux/arm64...                    ✓
  building darwin/amd64...                   ✓
  building darwin/arm64...                   ✓
  building windows/amd64...                  ✓
  committing...                              ✓
  tagging v0.2.7...                          ✓
  pushing...                                 ✓
  creating GitHub release...                 ✓
  uploading commitdog-linux-amd64...         ✓
  uploading commitdog-linux-arm64...         ✓
  uploading commitdog-darwin-amd64...        ✓
  uploading commitdog-darwin-arm64...        ✓
  uploading commitdog-windows-amd64.exe...   ✓
  uploading checksums.txt...                 ✓

  ✓ v0.2.7 released
  https://github.com/aysdog/commitdog/releases/tag/v0.2.7
```

every step registers an undo. if anything fails — network cut, GitHub API down, build error — commitdog rolls back every completed step in reverse. your repo is always left clean.

**version drift detection** — if your version file says `v0.2.5` but the latest git tag is `v0.2.6`, commitdog warns before touching anything.

---

## release targets

```sh
commitdog release config
```

```
  select build targets:

  enter numbers separated by spaces, e.g. 1, 2, 3

  1  linux/amd64        ✓
  2  linux/arm64        ✓
  3  darwin/amd64        
  4  darwin/arm64        
  5  windows/amd64       

  [1-5] toggle, [a] all, [enter] confirm, [q] quit › 3, 4
```

saved to `.commitdog` in your repo root. re-run anytime to change. if you skip config, commitdog will ask once — choose defaults or configure manually.

---

## pull requests

```sh
commitdog pr
```

on a feature branch — interactive diff viewer → create PR. on main — list all open PRs, pick one to review, merge (merge / squash / rebase), or close. deletes the remote branch after merge.

---

## git log

```sh
commitdog log
```

each branch gets its own RGB color. `j`/`k` to scroll, `a` to show all commits, `q` to quit.

---

## new project

```sh
commitdog init
```

creates a GitHub repo via API, runs `git init`, makes the first commit, sets the remote, and pushes — no browser needed.

---

## uninstall

```sh
# Linux / macOS
rm $(which commitdog) && rm -rf ~/.config/commitdog

# Windows (PowerShell)
Remove-Item "$env:USERPROFILE\AppData\Local\commitdog.exe"
Remove-Item -Recurse "$env:APPDATA\commitdog"
```

---

## contributing

open an issue first. if it fits — zero telemetry, pure stdlib, no external deps — we'll merge it.

rule #1: don't add telemetry. not "anonymous" telemetry. not a single ping.

MIT license.