# What works with what

Three things have to line up before a single command runs: a **transport** to
carry it, a **station** to run it, and a **controller** to publish it from. This
page is all three, and which combinations actually work.

The tables are generated from one source, so they cannot disagree with each
other. Where a row says something is unproven, that is the honest state rather
than modesty.

## Every transport is one of three shapes

This is the distinction the whole design rests on, and it decides more about an
estate's answer than any other fact on this page. The axis is one thing: **what
is held, and for how long.**

| | |
|---|---|
| **beacon** | A signal left where both can see it. You cannot reach the far side, the far side cannot reach you, and **both can reach one agreed place**. A *message* waits in the middle; nobody is ever connected |
| **flare** | Fired straight at a reachable endpoint. One burst, an answer, gone - a *transaction*, not a drop and not a standing line |
| **beam** | Held steady on the far station, live and two-way until it is torn down. The *connection itself* is what is held |

**Everything shipped is a beacon.** git, the relay, a file share, Azure Blob,
an object store and a bundle are all the same shape: something is left
somewhere, and collected later by the other side. The beam is designed and not
yet built, so nothing below carries it; the shapes are named together because
the vocabulary is one thing.

**The beam will need a compiled binary on the station, and almost nothing else
does.** Noise and a `wss` connection are not things `curl` and coreutils do, so
the beam's station side is designed as a Go component. That is worth knowing
before you plan around it, because the far side being plain readable bash is
often the reason heliograph is permitted at all.

What needs a binary today is one transport, on one station: the
[relay](/relay), on a **bash** station, which shells out to `heliograph-seal`
for the encryption. git, a file share, a bundle and an object store need none,
the PowerShell station needs none even for the relay, and a beacon or a flare
over any of those is a complete product - the same requests, the same gates,
the same logs. An estate that permits no compiled code loses two shapes, not
the tool.

The price of every far-side binary is a build you can reproduce and a checksum
you can check for yourself, which is on [provenance](/provenance).

### Why the shape matters more than the speed

The obvious difference is latency, and it is the less important one.

**A beacon keeps the gates where they belong.** The step is already on the
far side, the station reads a request that names it, and the station decides
whether to run it. `# heliograph-mode: read-only` is a property of a file the
operator can read before anything happens.

**On a flare the script travels with the request.** There is no git on the
far side to have planted it, so the caller ships the thing to be run - and at
that moment the mode header stops being a control and becomes *a claim the
caller makes about its own file*. What is left is the function key and an IP
allowlist. That is a real trade and [the flare page](/flare) makes it in
full; it is not a worse transport, it is a different security model.

**A beacon also needs nothing to be reachable, ever.** No inbound port, no
endpoint, no tunnel. That single property is what makes this permissible in
estates where a reverse connection would be a breach, and it is why [a raw TCP
transport was dropped](/security) rather than built.

## Pick what you have

```matrix
Every transport, station and controller, and which combinations run.
```
