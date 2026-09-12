# Security

## Reporting a vulnerability

Email <dan@dbhq.uk> rather than opening a public issue. Include what you found,
how to reproduce it, and what an attacker could do with it. You will get a first
response within 48 hours.

### What happens next

The timeline is Google Project Zero's, because it is the one the industry already
recognises and there is no reason to invent another.

A report is **urgent** when there is a credible way to exploit it today *and* the
damage would be material. Both, not either. Something anybody can trigger that
only wastes a few seconds of CPU is not urgent; something serious that nothing
can reach is not urgent either.

| | fixed within | advisory published |
|---|---|---|
| **urgent** | 7 days | 30 days after the fix, and never later than day 60 |
| **everything else** | 90 days | 30 days after the fix, and never later than day 120 |

**The 30 days between the fix and the advisory is for you.** It is there so an
operator can upgrade before the details are public. heliograph runs in estates
with slow change control, and publishing the day a patch exists would expose
exactly the people the patch was for.

If something is still unfixed when its deadline arrives, a **defensive notice**
goes out anyway: affected versions, what it lets an attacker do, how to spot it
and how to mitigate it, without a working exploit. You should not have to wait on
a fix to find out you are exposed.

**These are commitments rather than aspirations.** If one is missed, the advisory
says so and says why. A deadline that gets quietly extended is worth nothing.

### Findings get published

Every confirmed finding is published once its fix has shipped, **in full rather
than summarised**: affected versions, what it allowed, and the reasoning about
why the old design was the wrong shape. That last part is usually the only part
worth reading.

This is deliberate and it is not comfortable. heliograph's whole proposition is
that you can read it before you run it. A project making that claim while keeping
a private list of the times its code was wrong is not really making it.

What comes out: anything identifying somebody's estate, credentials, raw logs and
live topology. **Where something is removed, the advisory names the category**,
so you can tell a redaction from an argument that was never made.

### If you run your own

**You are not behind anybody in the queue.** Nobody gets earlier warning of a
product vulnerability by paying for something. Charging for privileged protection
against a flaw in code published to everybody would be the wrong way round.

There is no way to reach somebody who pulled a container anonymously, and
pretending otherwise would be a promise nobody could keep. So two things help:

- **watch releases on this repository.** Advisories are published as GitHub
  Security Advisories
- **write down the version or image digest you deployed.** Advisories name
  affected versions exactly, which only helps if you know what you are running

Fixes land on the current release. An older deployment may need an upgrade rather
than a patch, and the advisory will say which.

### Dependencies

Whether the flawed code can actually be reached decides what happens, not whether
the package shows up in a manifest.

- **reachable in a shipped path:** treated as a heliograph vulnerability, with an
  advisory stating the real exposure
- **a new flaw found upstream:** reported to that maintainer within one working
  day. Coordinating with them does not extend the deadlines above
- **present but demonstrably unreachable:** updated in the next release and noted
  there, without an advisory implying you are at risk when you are not

Linking to somebody else's CVE and saying nothing more is not much use, because
the question you have is whether *this* is exploitable.

## What this skill does

Heliograph debugs a machine you cannot log into, through an operator who cannot
debug it, using a git repository as the transport in both directions. That design
has a consequence worth stating before anything else:

**Everything heliograph writes is committed and pushed.** Logs, command output,
environment snapshots. Git history is permanent and visible to everyone with read
access to the repository. Treat the repository as the audience for every byte a
step prints.

### The execution model, and what bounds it

A step is arbitrary shell. That is the point of the tool - anything you can
express as a command can be measured on a machine you cannot reach - and no
amount of wrapping changes it. What follows is what bounds it, stated plainly so
nobody has to infer it from the code.

**The account is the credential boundary.** heliograph holds no credentials of
its own: no cloud auth, no API keys, no tokens beyond the git remote. So the
honest answer to "what could this do to the estate" is "whatever the account
running it could do", and that is the boundary to write down before an
evaluation. `run.sh`, `caprun.sh`, `station.sh` and `pigeonhole.sh` all **refuse to
run as root** unless `ALLOW_ROOT=1` says the image has no other user.

**A step declares what it is, in its own file.** Every step carries
`# heliograph-mode: read-only` or `# heliograph-mode: action` in its first 30
lines, and a step that declares neither does not run at all. An action needs
`CONFIRM=yes` as well. This used to be a list of step names, which meant
`cleanup-disk` was treated as a diagnostic whatever it did.

The declaration is a statement by the step's author, checked at the boundary. It
is not a sandbox: an author can declare `read-only` and then write `rm -rf`, and
nothing in a shell runner can prevent that. It makes the classification explicit
and machine-checked rather than inferred from a filename.

**The unattended loop is read-only by default.** `station.sh` and `pigeonhole.sh`
refuse an action step unless started with `--allow-actions` /
`PIGEONHOLE_ALLOW_ACTIONS=1`. The refusal is published to `station/status` with its
reason, so the far side learns within one poll rather than waiting out a round
trip.

**Optional pinning, for an estate that wants an allowlist.** `REQUIRE_PIN=1
./station.sh` runs only files whose sha256 the operator approved with
`./station.sh --pin`; a new or edited step is refused until it is approved again.
It is off by default because it makes every new step wait for the operator,
which is the relaying the loop exists to remove. It covers `run.sh`,
`caplib.sh`, `lib/*.sh` and `steps/*`. It does **not** cover `station.sh`, which
self-updates on pull.

### Evaluating it without giving it anything

There is nothing to give it, which makes a first evaluation unusually cheap:

1. Create an unprivileged user with no sudo. That account is the whole blast
   radius.
2. `bootstrap.sh` a scratch transport repo, private, on a host you already own.
3. `./run.sh env` - the baseline step. It reads; it changes nothing.
4. Read the log it committed. That log is the entire product.

No cloud credentials, no API keys, no agent identity, nothing to rotate
afterwards.

### Network

The skill makes no calls of its own beyond `git`. It pushes to and pulls from
whichever remote you configure - your repository, your host. There is no DBHQ
endpoint, no telemetry and no third-party service in the path.

### Credentials

Two separate secrets, handled differently.

**The git token**, for pushing over HTTPS. Resolved in this order:

1. `GIT_AUTH_HEADER` - a complete header, if you would rather build it yourself
2. `GIT_TOKEN` in the environment
3. The first line of `GIT_TOKEN_FILE`, then `./.git-token`, then `~/.git-token`

It is passed to git via `http.extraHeader`, **not** embedded in the remote URL.
That matters: a token in a remote URL ends up in `.git/config`, the reflog, and
every `git remote -v` anyone runs. This approach keeps it out of all three.

**The encryption passphrase**, for `secret.sh`, at
`~/.heliograph-passphrase` (override with `PASSPHRASE_FILE`). Written under
`umask 077` and `chmod 600`, so it never exists world-readable even briefly.

### Carrying a secret across the gap

`secret.sh` exists because sometimes a value has to reach the far side and no
channel exists between the two machines except the repository itself. The value
travels as **ciphertext in the repository**; the passphrase travels separately
through a human channel.

Consequences to understand before using it:

- **`secret.sh rm <name>` forgets the value, it does not erase the history.**
  The ciphertext stays in every clone and every commit that carried it. Rotate
  the underlying credential rather than assuming removal undoes exposure
- The strength of the scheme is the strength of the passphrase and the secrecy
  of the human channel that carried it
- `secret.sh key show` prints a fingerprint so both sides can confirm they hold
  the same passphrase without either transmitting it

### Log redaction is a safety net, not a guarantee

`cap_redact` masks two kinds of shape on their way into a committed log.

By **position**: `password=` / `token=` / `api_key=` / `client_secret=` /
`AccountKey=` style assignments, a credential carried in a URL, an
`Authorization:` header with any scheme, and PEM private key blocks.

By **shape**, where a vendor publishes a prefix precisely so that scanners can
recognise it: GitHub (`ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, `github_pat_`),
AWS access key ids (`AKIA`, `ASIA`), Slack (`xox…`), GitLab (`glpat-`), `sk-`
style API keys, and JWTs.

It is a regex filter over a stream. It will not catch a secret in a shape it does
not recognise, and it cannot unmask what a command chose to print in an unusual
format. **Never deliberately run a command that prints a secret**, and do not
treat redaction as permission to be careless. `REDACT=0` disables it when masking
is hiding output you genuinely need.

By design, `secret_name=pw-foo` is **not** masked - the name of a Key Vault
secret is useful and harmless - while `admin_password=...` is.

### On disk

- Installs into `~/.claude/skills/heliograph` or `~/.codex`
- Reads `~/.git-token` and `~/.heliograph-passphrase` if present
- Writes logs into the working repository, which are then committed

## Suited to

Air-gapped, client-owned and change-controlled estates, where the constraint is
that you have no interactive access and the operator has no debugging skill. It
is not a remote shell and gives no live access to the target - each exchange is
a commit.
