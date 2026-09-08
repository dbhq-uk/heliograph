# Secrets

Captured logs are committed and delivered, so **anything a command prints lands
in history permanently and cannot be unpublished.** Every rule here follows
from that one sentence.

## Name secrets, never read them

Listing the names of secrets settles *"does this exist here"*, which is almost
always the actual question. The value is not.

```bash
az keyvault secret list --vault-name kv-prod --query '[].name' -o tsv   # yes
az keyvault secret show  --vault-name kv-prod -n db-password            # no
```

If you genuinely need to know a value is *correct*, print its length or a
hash of it, never the value.

## `cap_redact` is a safety net, not a guarantee

Every captured line passes through a masking filter on its way to the log. It
catches credentials two ways.

**By position** - in a URL's userinfo, after `password=`, after `Bearer`. These
need careful anchoring, because the surrounding text is ordinary and the
pattern has to know where to stop.

**By shape** - `ghp_`, `github_pat_`, `AKIA`, `ASIA`, `xox`, `glpat-`, `sk-`, a
JWT's three dot-separated segments, a PEM private key header. Each of those is
a prefix a vendor publishes precisely so scanners can recognise it.

```
password=hunter2                    -> password=***REDACTED***
Authorization: Bearer eyJhbGciOi... -> Authorization: Bearer ***REDACTED***
https://ci:glpat-XXXX@git/x.git     -> https://ci:***REDACTED***@git/x.git
https://ghp_XXXX@github.com/o/r     -> https://***REDACTED***@github.com/o/r
```

`REDACT=0` turns it off when masking is hiding something you actually need to
read.

### What it deliberately does not mask

`secret_name=pw-foo` is left alone. The *name* of a vault secret is useful and
harmless; `admin_password=...` is not.

Ordinary output is left alone, and that is asserted rather than hoped for. A
redactor that eats evidence is its own failure - `listening on port 8443 and
exit code 0` must survive untouched.

### One trade-off taken with eyes open

`https://username@github.com/...` is a legitimate, non-secret form, and it is
masked anyway. Nothing in a line of text can tell a username from a token in
that position. The cost of hiding a username is a name the reader finds
elsewhere in seconds; the cost of missing a token is a live credential in a git
history that cannot be unpublished.

### And one it does not catch

A password containing `/` in a URL is not masked. Allowing `/` in the pattern
lets it run past a path segment and mask an `@` belonging to a path, and losing
evidence is the more expensive mistake. **Best effort, as the name says.**

Never deliberately run a command that prints a secret and rely on this.

## Sending a value the other way

Sometimes the far side needs a value from you - a licence key, a connection
string, a one-time password. `./secret.sh` in the transport repo carries it as
ciphertext.

```bash
./secret.sh key                        # define the passphrase on this machine
./secret.sh put db-conn                # encrypt a value into secrets/db-conn.enc
./secret.sh get db-conn                # decrypt it on the far side
```

The passphrase is defined **by a human on both machines** and never committed.
It lives outside the repo, mode 0600.

**It is transport, not storage.** The ciphertext is committed, so it is in that
history forever. Rotate anything sent this way once it has done its job, and
treat the encrypted file as a message rather than a vault.

## The transport repo

**Private, and its own repo.** Captured logs are committed to it, so everything
the operator's commands print lands in that history permanently.

Never bootstrap into a repo that holds anything else, and never into a public
one. The bootstrap reports rather than overwrites for the same reason.

The station's shipped `gitignore` covers the obvious hazards - `.git-token`,
passphrases, `*.pem`, `*.key`, `kubeconfig`, terraform state, `*credential*`.
It deliberately does **not** ignore `ops-logs/*.txt`: committing those is the
entire point of the repo.

## If something does leak

Rotate it first. A `git filter-repo` on a transport repo is worth doing, but it
is second - the credential is live from the moment it is pushed, and rewriting
history does not un-fetch it.
