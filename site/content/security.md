# Security

What heliograph refuses to do, what it gates, and what it cannot promise. This
page is written to be read by somebody deciding whether to permit it in a
regulated estate.

```diagram gates
Four gates, all failing closed, all in one place.
```

## It does not give you access you do not have

heliograph does not tunnel, proxy, or hold a connection open to a host you
control. There is nothing here to punch through a firewall with.

A raw TCP transport was considered and **dropped** for exactly that reason: a
persistent reverse connection from the far side to a listener you control is a
C2 channel by any blue team's definition. That sentence is a large part of why
this class of tool is permitted where it is, and it is not worth spending.

DNS tunnelling was declined for the same reason and more firmly - it is a
covert channel, and shipping one would get the product banned from the estates
it targets.

**Every command runs on the far side because somebody with legitimate access
chose to run it.** The operator clones a repo they can read and runs a script
they can read first.

## The four gates, all failing closed

**1. A step declares itself, or it does not run.** `# heliograph-mode:
read-only` or `action`, in the step's own file, read from the file about to be
executed. Missing or unrecognised refuses with exit 3.

Read **case-sensitively**, in both runners. `READ-ONLY` is not `read-only`, and
`YES` is not `yes` for gate 2 either. That is not pedantry: PowerShell compares
case-insensitively by default and bash does not, so until both were made strict
a state-changing step ran on one implementation and was refused by the other,
*from the same request*. A byte-order mark is refused too, by both, and said
plainly - three invisible bytes hide the declaration from a `sed` anchor, break
a shebang, and on Git-Bash make the file look non-executable, so every symptom
points somewhere other than the cause.

**2. An action needs `CONFIRM=yes`.** Carried in the request's `env:` line and
checked by the runner.

**3. The station must have been started with `--allow-actions`.**
`ALLOW_ACTIONS` defaults to `0`. The refusal is *published* to
`station/status` within one poll, so the far side learns in seconds rather than
waiting out a round trip - which is what makes a safe default affordable.

**4. Nothing runs as a privileged account.** Refused, not warned about, unless
`ALLOW_ROOT=1`. That is root on Unix and Administrator or SYSTEM on Windows;
the variable keeps one name, because an operator who has read this page should
not have to learn a second spelling to switch it off.

None of the four lives in a transport, so they cannot drift per channel. Three
of them - 1, 2 and 4 - live in the **runner**, which is what makes
`./run.sh <step>` by hand as gated as a request. Gate 3 lives in the **loop**,
because it is a property of how the station was *started*, and a runner invoked
by hand has no station behind it to ask. The CLI publishes requests and reads
logs; it gets no path around any of them.

**Both implementations carry all four.** The bash station and the [PowerShell
twin](/windows#the-powershell-station-for-a-box-with-no-bash) gate identically,
with the same exit codes, and the conformance suite tests the gates on both -
including that they fail *closed*, which is the only direction that matters.

### The account is the blast radius

This tooling holds no credentials of its own - no cloud auth, no API keys,
nothing but the transport's. So the honest answer to *"what could this do to
the estate"* is **whatever the account running it could do**, and that answer
is only useful if the account is not root.

Refused rather than warned about, because a warning in a captured log is read
after the run, by which time the run has happened. `ALLOW_ROOT=1` exists for
the appliance or minimal image that genuinely has no other user; the shipped
container and Kubernetes manifest both run as uid 1000 and need none of it.

### What the gates do not do

They do not stop an author declaring `read-only` and then writing `rm -rf`.
Nothing in a shell runner can. The gate makes the classification an explicit
statement in the file being run, checked at the boundary, instead of a guess
made from a filename.

For an estate that wants "runs only what I approved", `REQUIRE_PIN=1` refuses
any step whose file hash the operator has not approved. It is off by default
because it reintroduces the relaying this tool exists to remove.

## Claude Code permissions, sandboxes, and where the gates sit

Two searches bring people here, and both deserve a straight answer.

**Is heliograph a sandbox for Claude Code?** No. Claude Code's permission
prompts and its sandbox govern what the agent may do on the machine it is
running on: yours. heliograph never puts the agent on the far side. It
publishes a request, and a station somebody else started decides whether to run
it, through the four gates above. So there is nothing for heliograph to
sandbox. The agent's whole reach into the estate is one file that declares
itself read-only or an action, and a station that refuses anything else.

**Does `--dangerously-skip-permissions` change what it can do?** No. That flag
turns off the prompts Claude Code shows before acting on your machine. It says
nothing to the station. A step still has to declare its mode, an action still
needs `CONFIRM=yes`, the station still has to have been started with
`--allow-actions`, and root is still refused. The agent can skip its own prompts
and lose nothing but its own prompts; what runs on the far side is decided on
the far side, by the operator who started the loop.

The honest corollary: heliograph does not make skipping permissions safe on the
control side either. It never touches your machine's permissions, in either
direction.

## A request is a control channel, and is treated as one

The request names the step to run, so it is trusted by construction. Trusted is
still not a reason to hand it a shell.

- The `env:` line is refused outright if it contains `$`, a backtick, `;`, `&`,
  `|`, `<`, `>` or `(`. It is split the way a shell would split it, honouring
  quotes, and assigned as an array - so a value that got past the guard still
  could not execute
- It may not set `TRANSPORT`, `PUSH`, `REDACT` or `LOG_DIR`. Those control
  where the log goes, whether it is delivered at all, and whether secrets are
  masked in it. They are settled when the station is started, not per request.
  The check runs on the **parsed** assignments rather than on the raw line, so
  quoting cannot walk around it
- The transport name is validated before it becomes a filename that gets
  sourced

## The transport repo is private, and separate

Captured logs are committed to it, so everything a command prints lands in that
history permanently and cannot be unpublished.

**Never bootstrap into a repo that holds anything else, and never into a public
one.** `cap_redact` masks the obvious shapes and is a safety net, not a
guarantee - see [secrets](/secrets).

## The relay is outside the trust boundary, in both directions

A relay that could read logs would be a privacy problem. A relay that could
**forge a request** would have code execution inside every estate at once,
through a channel the estate installed deliberately. The second is the one that
matters.

Content is end-to-end encrypted with keys the relay never holds, and every
message is signed and verified before it is acted on. Nothing bespoke: age
primitives plus Ed25519. The relay server is [its own
repository](https://github.com/dbhq-uk/heliograph-relay) precisely so it is
publicly, obviously incapable of either. Details: [the relay](/relay).

## Reporting something

Security issues go to the address in
[`SECURITY.md`](https://github.com/dbhq-uk/heliograph/blob/main/SECURITY.md),
not to a public issue.

## What we cannot honestly claim

- **Nothing here has been penetration tested by a third party.**
- Nothing exercises a capture against a real remote machine in an adversarial
  setting. The behaviour that matters is what a log looks like after a round
  trip through someone else's terminal
- `cap_redact` is best-effort pattern matching. It will miss a secret shaped
  like ordinary text, and it is not a substitute for never printing one
- A step author with commit access to the transport repo can run anything the
  station's account can run. That is the design - it is what the tool is for -
  and the control is who has write access to that repo
