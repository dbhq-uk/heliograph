# Security

What heliograph refuses to do, what it gates, and what it cannot promise. This
page is written to be read by somebody deciding whether to permit it in
an estate nobody outside can log into.

```diagram gates
Four gates, all failing closed, all in one place.
```

## It does not give you access you do not have

Two of the three shapes never hold a connection open, and this section is
about them: heliograph does not tunnel, proxy, or hold a connection open to a
host you control. There is nothing here to punch through a firewall with. The
third, the beam, does hold one open, deliberately and under controls of its
own - covered below.

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

## The beam, and the blast radius of a held-open line

The beam is designed and not yet built; what follows is what it will do when
it lands. A beam is a live channel. While it is up, a step beam still runs
each line through `run.sh`, so the read-only gate and the captured log
survive - but a **raw** beam carries opaque bytes, and `run.sh` cannot see
inside them. That is interactive access to the account the station runs as,
gated once, at the door.

The controls are therefore at establishment, and there are three:

- a beam does not establish at all unless the station was started to allow one
- a raw beam needs a further, separate permission, because it is the one that
  removes the per-command gate
- an onward forward reaches only destinations the operator listed by name

A raw beam leaves a connection record rather than a captured log: class,
destination, peer, open and close times, and bytes each way. It is not the
content, and the page says so rather than implying otherwise.

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
- **It may not set anything that configures capture, delivery, redaction or
  identity.** Those decide where the log goes, whether it is delivered at all,
  whether secrets are masked in it, and who can read it - and they are settled
  when the station is started, not per request. That covers `TRANSPORT`, `PUSH`,
  `REDACT` and `LOG_DIR`, the gate variables, and **every transport's own
  configuration**: `RELAY_*`, `SHARE_*`, `PIGEONHOLE_*` and the rest.

  It is reserved by **prefix rather than by name**, because a transport is
  configured through the environment and a list of names would be outgrown by
  the next transport added. `tests/test-station-gate.sh` reads the pattern out
  of `station.sh` and fails if any variable a transport asks for is not covered
  by it, so the guard cannot quietly fall behind the code.

  **Both stations reserve the same set**, and the same test compares the two
  patterns rather than reading each and hoping. A control side cannot tell which
  implementation answered a request, so a gate that differed between them would
  mean the same request refused on one machine and honoured on another. The
  PowerShell station matches case-insensitively where the bash one does not, for
  a platform reason rather than a protocol one: Windows environment variable
  names are case-insensitive, so `transport=relay` there IS `TRANSPORT`.

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

## Who may command a station, and who may change that

Signature verification only tells you that a request was signed by a key the
station was told to trust. So the interesting question is not "is it signed" but
**who decides what the station trusts** - because anything that could add a key
could then sign legitimately, with nothing stolen and nothing forged.

A station may hold a **trusted set**: several keys, each belonging to one
person, any of which may author a request. Four rules govern it, and each does
one job.

**A change to the set is itself a signed document.** It travels over the
transport the estate already uses and is verified against the set as it stands.
There is no unsigned path and no permissive mode.

**Any trusted key may add or revoke any key except the anchor.** Offboarding one
engineer across forty estates cannot mean forty visits, so revocation is
something a colleague does remotely.

**The anchor changes only on the machine.** The anchor is the key the estate's
owner keeps, planted when the station is planted. No request can move it, add a
second member under its name, or revoke its key - not even a request signed by
the anchor's own key. A compromised key can therefore evict every other
engineer and cannot evict the owner, so recovery is a signed change from
whoever holds the anchor rather than a site visit.

**The station publishes its set.** The digest, the serial and every member's
fingerprint go into the status on every transition, and a full copy is written
to the transport. So an estate owner can audit who may command their machine
from their own transport, with the CLI, **without asking us** - and
`heliograph doctor` reports a set that differs from the one the control node
holds.

### What a service may and may not do

| may | may not |
|---|---|
| display the set a station reported | add a key |
| **propose** a change, as an unsigned draft a person signs on their own machine | revoke a key |
| record that a change happened, as an audit event | alter a change in transit |
| alert that a station's set differs from the expected one | be **required** for a legitimate change |

That last row matters twice. If a service were required, losing it would lock a
customer out of their own estate, **and** compromising it would be equivalent to
holding a key. A change is authored from a set and a key with no network on the
path, so a customer can administer their trusted set with the service
unreachable.

### Revocation is eventual, and that is worth stating

A station learns of a revocation **on its next poll**. Between a change being
sent and that poll, the revoked key still works.

An **already-running step is not interrupted**. Revocation removes the ability
to ask for the next one, not the ability to finish the current one. Terminating
an open session is a separate mechanism and does not exist yet.

For a station polling every five seconds that window is seconds; on a long
interval it is that interval. Neither is instant. "Revoked" read as "instant" is
the kind of assumption that gets discovered during an incident, which is why it
is written here rather than left to be inferred.

### What it needs on the far side

Verification is Ed25519, which bash and GNU coreutils cannot do. The two
stations resolve that differently, and the difference is worth knowing before
you plan a rollout:

- the **PowerShell station** needs nothing new. Its verification is managed C#
  already in the payload
- the **bash station** needs `heliograph-seal`, the single binary the relay
  transport already installs on the far side. A bash station on any other
  transport can use a trusted set by carrying that binary too

A bash station told to use a trusted set without it **refuses to start**, rather
than accepting everything quietly - a station that was told to verify and then
did not is one whose owner believes an assurance the mechanism is not providing.

Both implementations are held to the same bytes. `tests/fixtures/trust-vectors.json`
pins the canonical encodings, the digests, the signing input and the refusal
sentences, and the PowerShell side is checked against it - because a control
side cannot tell which implementation answered, and two stations disagreeing
about who may command a machine would make an owner's audit right about half
their estate.

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
- **Revocation is eventual, never instant.** A station acts on it at its next
  poll, and a step already running is not interrupted. See above
- A trusted set is **opt-in and not yet the default**. A station started without
  one behaves as it always has: it accepts whatever its transport verifies,
  which for the relay is one recorded peer and for git is whoever has write
  access to the repo
