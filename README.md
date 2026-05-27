<div align="center">

# commitdog

![license](https://img.shields.io/badge/license-MIT-brightgreen?style=flat-square) ![go](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=flat-square&logo=go&logoColor=white) ![platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey?style=flat-square) ![telemetry](https://img.shields.io/badge/telemetry-none-4ade80?style=flat-square) ![part of](https://img.shields.io/badge/part%20of-aysdog-orange?style=flat-square)

git, without the grief.

commit messages, branch management, sync, PR, merge, release — all from one tool. pure Go stdlib. no AI. no telemetry. works with GitHub, GitLab, Gitea, and Forgejo.

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

```sh
commitdog setup
```

picks which platform to configure, saves your email and token. run it once per platform — you can have tokens for all four stored at the same time.

```
  which platform token do you want to configure?

  1  github
  2  gitlab
  3  gitea
  4  forgejo
  5  remove a platform

  [1/2/3/4/5] pick › 1
```

stored at `~/.config/commitdog/config.toml` with `0600` permissions. never leaves your machine except when calling the platform API.

option `5` removes a platform — wipes the token from global config, removes the git remote from the current repo, and cleans it out of `.commitdog`. if it was your primary, you'll be told to run `commitdog init` again.

**token scopes needed:**

| platform | scope |
|----------|-------|
| GitHub | `repo`, `write:org`, `read:user` — classic PAT at [github.com/settings/tokens](https://github.com/settings/tokens) |
| GitLab | `api` — legacy token at [gitlab.com/-/profile/personal_access_tokens](https://gitlab.com/-/profile/personal_access_tokens) |
| Gitea / Forgejo | `repository`, `organization`, `user` read+write — under settings → applications |

---

## commands

| command | what it does |
|---------|-------------|
| `commitdog` | stage all, suggest commit messages, pick, commit, push |
| `commitdog <file>` | stage a specific file only |
| `commitdog -gh` | commit and push to GitHub |
| `commitdog -gl` | commit and push to GitLab |
| `commitdog -gt` | commit and push to Gitea |
| `commitdog -fg` | commit and push to Forgejo |
| `commitdog init` | create repo on your platform, git init, first commit, first push |
| `commitdog setup` | configure email and platform token |
| `commitdog branch` | interactive branch menu |
| `commitdog switch` | jump to branch switcher |
| `commitdog merge` | merge a branch with diff preview + auto-generated commit message |
| `commitdog pr` | create PR with auto-generated title and description · list/review/merge PRs on main |
| `commitdog sync` | fetch + pull rebase + push — auto-recovers on errors |
| `commitdog stash` | save, pop, or drop stashes interactively |
| `commitdog revert` | pick from last 5 commits and revert |
| `commitdog secrets` | scan full commit history for leaked secrets |
| `commitdog release` | bump version, build binaries, release notes, tag, push, create release |
| `commitdog release --all` | release to primary + all mirrors |
| `commitdog release config` | configure which platforms to build for |
| `commitdog release --changelog-only` | preview grouped changelog since last tag |
| `commitdog version` | print version |
| `commitdog help` | show help |
| `commitdog --update` | update to latest release |
| `commitdog --uninstall` | remove commitdog completely |

---

## commit

```sh
commitdog
```

```
  nothing staged. staging all changes...

  suggestions:
  1  feat(auth): add refreshToken and verifyToken
     adds refreshToken, verifyToken, validateSession
  2  feat: implement refreshToken in auth
     adds refreshToken, verifyToken, validateSession
  3  feat: add functions to auth
     adds refreshToken, verifyToken, validateSession

  [1/2/3] pick, [e] edit, [q] quit › 1

  ✓ committed: feat(auth): add refreshToken and verifyToken
  ✓ pushed to origin/main
```

suggestions come from your actual diff — function names, file types, scope inference. no AI, no network call. pick `[e]` to edit before committing. press `[q]` to abort — staged changes are automatically removed.

**language support:** Go, JavaScript, TypeScript (including interfaces, types, enums), Python, Ruby, Rust, Java, Kotlin, PHP, C/C++, Vue, Svelte, Shell, HTML, CSS, SQL, Dockerfile.

when multiple files are committed together, the body lists all changed functions:

```
  adds listGitLabMRs, createGitLabMR, mergeGitLabMR, deleteGitLabBranch,
    listGiteaPRs, createGiteaPR, mergeGiteaPR, deleteGiteaBranch,
    listForgejoPRs, createForgejoPR, mergeForgejoPR, deleteForgejoBranch
  removes runPRList
```

if you commit to a mirror platform without changes to stage, commitdog detects unpushed commits and tells you:

```
  up to date with github
  gitea has unpushed commits — run 'commitdog -gt' to push
```

---

## secret detection

before showing suggestions, commitdog silently scans your staged diff. if it finds a secret it blocks and shows exactly where:

```
  ✗ possible secret detected in staged changes:

  · AWS access key  in config.go
    var awsKey = "AKIAIOSFODNN7EXAMPLE"

  commit anyway? this will push secrets to your remote. [y/N] ›
```

catches AWS keys, GitHub tokens, GitLab tokens, private keys, Stripe keys, Slack tokens, generic passwords and API keys. skips `_test` files automatically.

---

## pull requests

```sh
commitdog pr
```

**on a feature branch** — shows the diff, then auto-generates a PR title and full description from your branch commits, grouped by type:

```
  14 commits on dev

  title: Add PR support for GitLab, Gitea, Forgejo (+6 more)

  ─────────────────────────────────
  ## what's changed

  ### features
  - Add PR support for GitLab, Gitea and Forgejo
  - Add auto-generated PR description from commits

  ### bug fixes
  - Fix currentAuthHeader using wrong platform token
  ─────────────────────────────────

  [enter] use as-is  [e] edit  [t] edit title  [q] cancel ›
```

`[e]` opens your `$EDITOR` with the full content — title on line 1, description below. edit freely, save, done.

**on main/master** — lists all open PRs, pick one to review diff, merge (merge / squash / rebase), delete remote branch after merge. works on GitHub, GitLab, Gitea, and Forgejo.

---

## merge

```sh
commitdog merge
```

shows a diff preview of the branch, then merges. after merge, auto-generates a detailed commit message from all commits on that branch:

```
  ─────────────────────────────────
  merge dev → main: add PR support for all platforms, fix 8 issues (23 commits)

  features
    · Add PR support for GitLab, Gitea and Forgejo
    · Add auto-generated PR description from commits
    · Add platform remove from config

  bug fixes
    · Fix currentAuthHeader using wrong platform token
    · Fix push not retried after auto-recovery
    · Fix hasUnpushedCommits for never-pushed remotes
  ─────────────────────────────────

  [enter] use this message  [e] edit  [s] skip ›
```

`[e]` opens in your `$EDITOR`. `[s]` keeps git's default merge commit.

---

## new project

```sh
commitdog init
```

asks which platform, creates the repo via API, runs `git init`, makes the first commit, sets the remote, and pushes. no browser needed.

if the repo is already configured, shows options to manage it:

```
  this repo is configured for gitea.
  mirrors: github

  1  change platform
  2  add mirror
  3  remove mirror
  4  cancel
```

**removing a mirror** disconnects the remote locally, updates `.commitdog`, and shows the settings URL to delete the repo on the platform manually — commitdog never deletes repos automatically.

**changing primary** checks that the new platform has a token configured first. if it does, it updates the `origin` remote URL automatically using your stored PAT — no URL pasting needed.

per-platform email is set automatically per repo via `git config --local` — your global git email stays untouched.

---

## mirrors

push to multiple platforms at once:

```sh
commitdog release --all     # release to primary + all mirrors
commitdog -gh               # push to github (primary or mirror)
commitdog -gt               # push to gitea (primary or mirror)
```

add a mirror through `commitdog init` → `add mirror`. the URL is detected automatically from your stored token — no copy-pasting. `.commitdog` is updated before the push, so if the push fails the mirror is still registered and will be tried next time.

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

if `origin` is missing but another platform remote is detected, commitdog verifies the repo exists via API, shows you what it found, and asks before connecting — with a clear warning about what that means for future pushes.

other auto-recoveries: non-fast-forward, stale remote tag, missing upstream, protected branch. after any auto-fix, the push is automatically retried.

---

## release with atomic rollback

```sh
commitdog release
```

```
  detected: Go  ·  current version: v0.2.9

  1  patch  →  v0.3.0
  2  minor  →  v1.0.0
  3  major  →  v2.0.0
  4  custom

  [1/2/3/4/q] pick › 1

  ─────────────────────────────────
  ## what's new in v0.3.0

  commitdog now supports GitLab, Gitea, and Forgejo — same workflow,
  same commands, different platform.

  ### features
  - Add PR support for GitLab, Gitea and Forgejo
  - Add auto-generated PR description and merge commit message

  ### bug fixes
  - Fix currentAuthHeader using wrong platform token
  - Fix push not retried after auto-recovery
  ─────────────────────────────────

  [enter] use as-is  [e] edit  [q] cancel ›

  release v0.2.9 → v0.3.0? [y/n] › y

  bumping version in main.go...              ✓
  building linux/amd64...                    ✓
  building linux/arm64...                    ✓
  building darwin/amd64...                   ✓
  building darwin/arm64...                   ✓
  building windows/amd64...                  ✓
  committing...                              ✓
  tagging v0.3.0...                          ✓
  pushing...                                 ✓
  creating GitHub release...                 ✓
  uploading commitdog-linux-amd64...         ✓
  uploading commitdog-linux-arm64...         ✓
  uploading commitdog-darwin-amd64...        ✓
  uploading commitdog-darwin-arm64...        ✓
  uploading commitdog-windows-amd64.exe...   ✓
  uploading checksums.txt...                 ✓

  ✓ v0.3.0 released
  https://github.com/aysdog/commitdog/releases/tag/v0.3.0
```

release notes are generated from your commits since the last tag — grouped by features, bug fixes, security, removed — with an intro line. press `[e]` to open in `$EDITOR` before publishing.

every step registers an undo. if anything fails — network cut, API down, build error — commitdog rolls back every completed step in reverse. mirror releases use separate undo chains so a mirror failure never rolls back the primary release.

**platform support** — release works the same across GitHub, GitLab, Gitea, and Forgejo. GitLab uses Generic Packages + Release Links. Gitea and Forgejo upload assets in batches to stay within API limits.

---

## release targets

```sh
commitdog release config
```

```
  select build targets:

  1  linux/amd64        ✓
  2  linux/arm64        ✓
  3  darwin/amd64       ✓
  4  darwin/arm64       ✓
  5  windows/amd64      ✓

  [1-5] toggle, [a] all, [enter] confirm, [q] quit ›
```

saved to `.commitdog` in your repo root. re-run anytime to change.

---

## platform support

| platform | init | push | release | PR / MR |
|----------|------|------|---------|---------|
| GitHub   | ✓    | ✓    | ✓       | ✓       |
| GitLab   | ✓    | ✓    | ✓       | ✓       |
| Gitea    | ✓    | ✓    | ✓       | ✓       |
| Forgejo  | ✓    | ✓    | ✓       | ✓       |

HTTPS push uses `Authorization: Basic base64(oauth2:TOKEN)` via `http.extraHeader`. the token is never written to `.git/config`.

---

## uninstall

```sh
commitdog --uninstall
```

or manually:

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
