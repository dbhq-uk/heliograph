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

The same loop, driven from an agent. [Claude Code](/claude-code) and
[Codex](/codex) each have a page; Cursor, Copilot, Windsurf, Gemini and Cline
install through skills.sh below, and anything that speaks MCP can use
[the MCP server](/mcp) instead.

```
/plugin marketplace add dbhq-uk/marketplace
/plugin install heliograph@dbhq         # Claude Code
./install-codex.sh                      # Codex, from a clone
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

**Requirements:** bash 4 or newer, git, and whatever the step itself invokes.
Alpine and busybox work with nothing added. A GNU `sed` is preferred but not
required, and the preflight says exactly what you give up without one.

**macOS needs a bash installing.** It still ships 3.2 at `/bin/bash` - the last
GPLv2 release, from 2007 - and the station uses `declare -A` and `${var^^}`,
neither of which 3.2 has, so the preflight refuses it and will not start:

```bash
brew install bash
```

Nothing else about a Mac is unusual. The one thing to know is that a
**LaunchAgent does not inherit your PATH**: it gets
`/usr/bin:/bin:/usr/sbin:/sbin`, where the only bash is 3.2. `service.sh
install` resolves an absolute path to a newer one and writes it into the plist,
so this is handled - but it is why a Mac station has to be installed with
`service.sh` rather than by hand-writing a plist that says `/bin/bash`.

## Prove it will work before committing to anything

```bash
./start.sh --check    # changes nothing at all
```

It is what gets run on a node where nobody is permitted to alter anything yet,
so the answer to "will this work here" can be had before asking for permission.
