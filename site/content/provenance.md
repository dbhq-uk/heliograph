# Provenance

The argument for heliograph is that you can read it before you run it. That
argument only reaches the machine in front of you if the binary you are holding
is the source you read, and the only way to know is to build it yourself and
compare.

This page is the command for doing that, and an honest list of what it does not
yet cover.

## Reproduce a released binary

```bash
git clone --depth 1 --branch v0.4.0 https://github.com/dbhq-uk/heliograph
cd heliograph
packaging/reproduce.sh v0.4.0
```

You need bash, git and any Go 1.21 or newer. The script pins the toolchain
itself: it reads the version out of `go.mod` and sets `GOTOOLCHAIN`, so Go
fetches the exact compiler this project builds with and verifies it against the
checksum database. If you would rather not trust your own Go installation at
all, the same script runs in the pinned container:

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.27.1 packaging/reproduce.sh v0.4.0
```

Then compare against what was published:

```bash
curl -sSLo /tmp/published \
  https://github.com/dbhq-uk/heliograph/releases/download/v0.4.0/SHA256SUMS
cd dist && sha256sum --ignore-missing -c /tmp/published
```

Every line should say `OK`. If one does not, the binary in the release is not
the source in the tag, and that is worth telling us about at
[security@dbhq.uk](mailto:security@dbhq.uk).

**This holds from v0.4.0 onwards.** Earlier releases were built by a workflow
that did not pin the Go patch version and did not disable Go's VCS stamping, so
their hashes are not reproducible and nothing here claims they are.

## Why it needs a script rather than a `go build` line

Four things make the output deterministic, and each of them was added because a
build without it produced different bytes for no visible reason:

| | |
|---|---|
| `-trimpath` | the source path is otherwise compiled in, so `/home/you/heliograph` and `/src` produce different binaries |
| `-buildvcs=false` | Go stamps the commit, its time and a dirty flag into the binary by default, and omits them silently where there is no `.git`. The same source in a checkout and in a tarball of that checkout gave two different hashes |
| the toolchain pin | a Go **patch** release changes the compiler, so `1.27` is not a pin and `1.27.1` is |
| a cleared environment | `GOFLAGS`, `GOEXPERIMENT` and `GOAMD64` all change code generation, and all three can be set without anybody remembering they are |

The release workflow runs that same script. It is one file with two callers on
purpose: when the documented command and the published artefact are built by two
pieces of copied YAML, they differ by one flag eventually, and the first person
to notice is a stranger who concludes the source is not the product.

Every pull request builds the whole matrix twice - once in the checkout, once
from a copy at a different path with no `.git` - and fails if the two disagree.

## What is not covered, and is not being claimed

- **The `.mcpb` bundles** are zip archives. The file list, order, timestamps and
  modes are fixed, but zip compression is whatever the local zlib does, and
  builds of Python that link zlib-ng compress differently. The reproducible
  artefact is the binary inside the bundle.
- **The container images** are not byte-reproducible. `ghcr.io/dbhq-uk/heliograph`
  carries a [build provenance attestation](https://github.com/dbhq-uk/heliograph/attestations)
  instead, which proves which workflow and which commit produced the image
  without proving the bytes can be arrived at twice.
- **Signatures.** The release workflow signs `SHA256SUMS` with Sigstore keyless
  signing, and no release has been through it yet, so there is no signature to
  verify today. The section below says what will be there.
- **The hosted relay.** The Worker that runs at `heliograph-relay.dbhq.uk`
  reports the commit it believes it is at `/version`, which is detection rather
  than provenance: a version string is a claim a deployment makes about itself.
  A reproducible bundle hash for the Worker is being worked in
  [dbhq-uk/heliograph-relay](https://github.com/dbhq-uk/heliograph-relay). Until
  it lands, run [your own relay](/relay) if the deployed one being provably the
  published source is something your estate needs.

## Signatures, when there are some

There is no signing key, deliberately. The release workflow uses Sigstore
keyless signing: `cosign` gets a certificate valid for ten minutes, bound to the
workflow's own OIDC identity, and the binding is written to the public Rekor
transparency log. So what a verifier checks is an identity - "signed by the
release workflow of `dbhq-uk/heliograph`, at this tag" - rather than "signed by
a key somebody holds".

```bash
cosign verify-blob \
  --signature SHA256SUMS.sig --certificate SHA256SUMS.pem \
  --certificate-identity "https://github.com/dbhq-uk/heliograph/.github/workflows/release.yml@refs/tags/v0.4.0" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  SHA256SUMS
```

The trade is worth stating: there is no key to steal, to rotate or to explain
the custody of, and in exchange verification is an online check against
Sigstore. An air-gapped verifier can still reproduce the binary and compare
hashes, which is the part that does not need anybody's signature.

## The one binary on the far side

Everything a station runs is plain bash or plain PowerShell, planted as source,
readable before it is run - with one exception, and it is the reason the
checksums above matter beyond the CLI.

The [relay transport](/relay) on a bash station shells out to
`heliograph-seal`, a compiled Go binary, because the relay is encrypted end to
end so that we cannot read your logs, and X25519, ChaCha20-Poly1305 and Ed25519
are not things `curl` and coreutils do. A bash implementation of them would be
bespoke cryptography, which is refused outright.

So the station verifies it rather than trusting it:

```bash
RELAY_SEAL_SHA256=<the line from the release's SHA256SUMS>
```

With that set, `transports/relay.sh` refuses to start unless the binary on disk
hashes to that value, and the value comes from somewhere other than the machine
holding the binary. That check is only worth something if you can arrive at the
number yourself, which is what this page is for.

**Every other transport needs no binary at all.** git, a file share, a bundle
and an object store are pure bash on the station side, and a
[beacon or a flare](/matrix) over any of them is a complete product. The list of
compiled programs a station may ever be given lives in
[`station/FAR-SIDE-BINARIES`](https://github.com/dbhq-uk/heliograph/blob/main/station/FAR-SIDE-BINARIES),
adding to it is a deliberate edit that CI enforces, and reproducible builds are
the price of being on it. An estate that permits no compiled code loses two
shapes, not the tool - and even the relay has a way through, because the
PowerShell station ships the same construction as source and compiles it at
startup.
