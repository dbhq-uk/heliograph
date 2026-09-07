# Transports

A transport is the channel a request goes out on and a log comes back through.
The loop is identical whichever you pick: same request format, same gates, same
log. That is deliberate, and it is what lets you change transport without
relearning the method.

## Choosing one

**Measure before reaching past git.** Git is better when git works, and an
image pull succeeding proves nothing: a container platform pulls on its own
side, so a container can start cleanly on a host with no network at all.

| transport | reach for it when | status |
|---|---|---|
| **git** | the far side can reach a git host | works |
| **file share** | both machines mount the same directory | works |
| **bundle** | nothing crosses the gap but a person | works |
| **relay** | there is no git host, no storage, no share | works |
| **object store** | S3 or Azure Blob is permitted where git is not | designed |

## git

The default. A private repository is the transport in both directions: you push
a request, the station pushes back a status and a log.

```bash
heliograph init payments --dir ~/transport/payments
```

**Two writers, one branch.** The station pushes far more often than you do: a
status commit on every transition, a progress snapshot every 60 seconds, and the
log itself. So the remote moves while you are writing the next request and a
plain push is rejected. That is not a fault, it is the design working, and the
CLI rebases rather than forcing. A force push here would destroy the evidence
the loop exists to deliver.

## File share

The cheapest transport there is, and it covers a real population: an estate that
will not open an egress path, will not provision a storage account and will not
permit a git host quite often already has a share both machines mount, because
that is how everything else in the estate moves files.

```bash
heliograph init ops --transport share --dir /mnt/ops --scope dns-timeouts
```

**The mount is the credential**, which is also the whole security model. Anyone
who can write to the share can queue a request, so it must be scoped as tightly
as the account the station runs as.

`--scope` is one directory per investigation, so two do not overwrite each
other. Requests are written with write-then-rename: a station polling the
directory can read at any instant, and a partially written request is one with
no `id:` yet, or worse a truncated `env:`, and it would be acted on.

## Bundle

The only thing that makes "air-gapped" literally true rather than nearly true.
Everything else still needs some path between the two machines, even if it is a
storage account nobody can route to directly.

```bash
heliograph init air --transport bundle --dir ~/bundles
heliograph send net-probe            # writes request-<id>.hgb
```

Carry the file. Run it. Carry the log back.

It is not a loop and does not pretend to be: a run takes as long as it takes
somebody to walk. The value is that the format, the gates and the log are
identical to every other transport, so the method survives the walk.

## Relay

The only transport that needs no estate infrastructure at all. Both sides dial
**out** over ordinary HTTPS and meet at a server neither of them trusts.

```bash
heliograph init payments --transport relay \
  --url https://heliograph-relay.dbhq.uk --estate payments
```

**The relay cannot read your logs, and cannot make a station run anything.** The
second half is the one that matters: a relay able to forge a request would have
code execution inside every estate at once, through a channel the estate
installed deliberately. Everything is sealed and signed before it leaves, and
verified before it is acted on.

Nothing bespoke: X25519, HKDF-SHA256, ChaCha20-Poly1305 and Ed25519. The server
holds no keys, and its source is public precisely so that claim can be checked
rather than trusted.

It is the one transport that needs a binary on the far side. `openssl enc`
refuses AEAD ciphers outright, so a shell implementation would hand-assemble
encrypt-then-MAC and key agreement across openssl version differences, which is
where crypto bugs live and where they are silent. `heliograph-seal` does the
sealing and no networking at all. Every other transport stays pure bash.

Run your own, or use the hosted one:

```bash
docker run -p 8080:8080 \
  -e HELIOGRAPH_RELAY_ESTATES="payments:$CTL:$STN" \
  ghcr.io/dbhq-uk/heliograph-relay:latest
```

## What a transport must implement

Six verbs on the station side, so the loop never learns which one it is talking
to and the read-only gates live in one place rather than one copy per transport.

```
tp_fetch_request     emit the request document
tp_fetch_request_live  the same, DURING a run, without touching the working tree
tp_fetch_self        bring a newer station payload, if this transport can
tp_put_status        publish a status document
tp_put_progress      publish a partial-log snapshot
tp_check             can this station reach the transport at all
```

`tp_capabilities` reports which optional verbs a transport actually offers, and
the station says at **start** what it will not be able to do later.

That last part was learned rather than designed. The loop self-updates: a pull
brings a newer `station.sh` and it re-executes into it, which is what lets a fix
take effect without telling the operator to restart. Git gives that away free
and nothing else does. A transport that could not do it, and did not say so,
would be a station nobody could fix without finding a human on the far side.
