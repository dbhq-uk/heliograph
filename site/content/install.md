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

## The far side

Nothing to install. The station is
[dbhq-uk/heliograph-skill](https://github.com/dbhq-uk/heliograph-skill): plain
bash, no interpreter, no packages, no credentials of its own.

That is not a convenience, it is the proposition. On a locked-down box,
installing anything is its own change request, and a tool that needs a runtime
is a tool that never gets approved.

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
