# Install

## The control side

A single static binary, no runtime.

```bash
curl -sSL https://github.com/dbhq-uk/heliograph/releases/latest/download/heliograph-linux-amd64 \
  -o /usr/local/bin/heliograph && chmod +x /usr/local/bin/heliograph
```

Replace `linux-amd64` with `linux-arm64`, `darwin-amd64`, `darwin-arm64`,
`windows-amd64` or `windows-arm64`. Checksums are published with every release
as `SHA256SUMS`.

From source, if you would rather:

```bash
go install github.com/dbhq-uk/heliograph/cmd/heliograph@latest
```

## The agent skill

The same loop, driven from Claude Code, Codex, Cursor and friends. See
[Claude Code](/claude-code) for what it does.

```
/plugin marketplace add dbhq-uk/marketplace
/plugin install heliograph@dbhq         # Claude Code
npx skills add dbhq-uk/heliograph       # any agent, via skills.sh
```

The skill drives the binary above, so install both.

## The far side

Nothing to install, ever. The station is plain bash - no interpreter, no
packages, no credentials of its own - and `heliograph bootstrap` plants the
copy the binary was built with into your transport repo. The source is
[`station/bash/`](https://github.com/dbhq-uk/heliograph/tree/main/station/bash),
readable before you run it.

That is not a convenience, it is the proposition. On a locked-down box,
installing anything is its own change request, and a tool that needs a runtime
is a tool that never gets approved.

The operator's whole job:

```bash
git clone <your-private-transport-repo> transport
cd transport
./start.sh
```

**Requirements:** bash, git, and whatever the step itself invokes. Stock macOS,
Alpine and busybox all work with nothing added. A GNU `sed` is preferred but not
required, and the preflight says exactly what you give up without one.

## Prove it will work before committing to anything

```bash
./start.sh --check    # changes nothing at all
```

It is what gets run on a node where nobody is permitted to alter anything yet,
so the answer to "will this work here" can be had before asking for permission.
