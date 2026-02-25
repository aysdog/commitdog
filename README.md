# commitdog

zero-bs commit message generator. reads your staged diff, suggests 2-3 conventional commit messages, you pick one, done.

no AI. no network. no telemetry. single binary.

## install

```bash
curl -sL install.aysdog.dev/commitdog | sh
```

or build from source:

```bash
git clone https://github.com/aysdog/commitdog
cd commitdog
make install
```

## usage

```bash
git add .
commitdog
```

```
  suggestions:

  1  feat(auth): add refreshToken and verifyToken
  2  feat: implement refreshToken in auth
  3  feat: add 2 new functions to auth module

  [1/2/3] pick, [e] edit, [q] quit › 1

✓ committed: feat(auth): add refreshToken and verifyToken

  push to origin/main? [y/n] › y
  ✓ pushed to origin/main
```

## how it works

- runs `git diff --staged` internally
- parses changed files, added/removed function names, patterns
- generates suggestions using conventional commits format
- no network calls — ever
- no data leaves your machine — ever

## security

- all git commands use `exec.Command` with explicit args — no shell, no injection
- diff capped at 200KB — no memory bombs
- commit message sanitized before use
- branch/remote names validated against allowlist before push
- zero external dependencies — stdlib only

## build

```bash
make build          # local binary
make release        # all platforms in dist/
make test           # run tests
```

## philosophy

aysdog tools do one thing, do it well, and get out of the way.
zero telemetry. self-hostable. MIT licensed.

don't add telemetry to forks. that's the one rule.