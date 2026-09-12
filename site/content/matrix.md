# What works with what

Three things have to line up before a single command runs: a **transport** to
carry it, a **station** to run it, and a **controller** to publish it from. This
page is all three, and which combinations actually work.

The tables are generated from one source, so they cannot disagree with each
other. Where a row says something is unproven, that is the honest state rather
than modesty.

## Every transport is one of two shapes

This is the distinction the whole design rests on, and it decides more about an
estate's answer than any other fact on this page.

| | |
|---|---|
| **pigeonhole** | A dead letter drop. You cannot reach the far side, the far side cannot reach you, and **both can reach one agreed place**. Both sides dial out; neither ever accepts a connection |
| **intercom** | You can reach the station's endpoint directly, so there is no drop in the middle. Unusual, because the whole tool exists for when you cannot |

**Everything shipped is a pigeonhole.** git, the relay, a file share, Azure
Blob, an object store and a bundle are all the same shape: something is left
somewhere, and collected later by the other side. The words come from the
station itself - `pigeonhole.sh` and `intercom.sh` have carried them since
before this page existed.

### Why the shape matters more than the speed

The obvious difference is latency, and it is the less important one.

**A pigeonhole keeps the gates where they belong.** The step is already on the
far side, the station reads a request that names it, and the station decides
whether to run it. `# heliograph-mode: read-only` is a property of a file the
operator can read before anything happens.

**On an intercom the script travels with the request.** There is no git on the
far side to have planted it, so the caller ships the thing to be run - and at
that moment the mode header stops being a control and becomes *a claim the
caller makes about its own file*. What is left is the function key and an IP
allowlist. That is a real trade and [the intercom page](/flare) makes it in
full; it is not a worse transport, it is a different security model.

**A pigeonhole also needs nothing to be reachable, ever.** No inbound port, no
endpoint, no tunnel. That single property is what makes this permissible in
estates where a reverse connection would be a breach, and it is why [a raw TCP
transport was dropped](/security) rather than built.

## Pick what you have

```matrix
Every transport, station and controller, and what each one can work with.
```

## Two things the matrix cannot show you

**A transport needs both halves.** The object store and the bundle are
implemented on your side and have no station side at all, so the combination
cannot work however the rows line up. That is why the transport table has a
column for each side rather than one status.

**Proven is not the same as written.** A station that has never run is a
template that looks authoritative, and shipping twenty of those would spend the
credibility of the ones that work. [The host contract](/hosts) is published so
you can judge an unlisted host yourself.

What might be added next, what is deliberately refused, and why, is on
[the roadmap](/roadmap).
