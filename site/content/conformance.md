# The capture contract

There is one specification of the capture pattern, and this is it. Not a
description of it - an executable one, in `tests/conformance/`.

It exists because the capture is the thing heliograph is most careful about,
and because its failure mode is invisible. A broken capture does not crash. It
produces a log that reads perfectly and cannot answer the only question it was
written to answer.

## Run it

```bash
./tests/conformance/conformance.sh tests/conformance/drivers/bash.sh
./tests/conformance/conformance.sh tests/conformance/drivers/mutant.sh   # must FAIL
```

A **driver** supplies the implementation; the suite supplies the properties.
Nothing in the suite may reference `caplib.sh`, `run.sh` or any path inside the
station payload - the moment it does, it stops being a specification and
becomes a second copy of one implementation.

`drivers/mutant.sh` is deliberately broken and the suite asserts that it
**fails**. A test suite nobody has watched fail is a suite nobody knows works.

## The nine properties

**1. Every captured line carries a distinct UTC timestamp.**
Three lines a second apart must produce three different stamps. This is the
busybox failure and the reason the whole suite exists: a `sed` without `-u`
buffers, every line of the block is stamped when the buffer flushes, and the
log reads perfectly while being useless.

**2. A real gap in the source appears as a gap in the log.**
Distinct is not enough. Stamps could be distinct and still wrong, by being
applied when a line is read from a buffer rather than when it was produced. A
three-second sleep must show as a three-second gap.

**3. The step's real exit code survives the pipeline.**
The capture is a pipeline, so `$?` is the exit of the last stage and is almost
always 0. Getting this wrong publishes every failed run as a success - the most
expensive possible defect here, because it wastes a round trip through somebody
who cannot debug the machine. Both 42 and 0 are asserted, so the check cannot
be stuck on failure.

**4. A failed run still writes a complete log.**
Same footer, real exit code, `RESULT: FAILED`, and the output produced before
the failure. A passing run says `OK`, so `RESULT` is not hardcoded.

**5. An undeclared step refuses, exit 3, having written nothing.**
With a control: the same runner, the same invocation, a step that *does*
declare itself, which must run. Without that, the property would pass just as
well if the runner refused everything.

**6. Running as the privileged account refuses, exit 5.**
Simulated rather than actually run as root, and **the driver supplies the
simulation** - the suite asks the question and the platform answers it. On Unix
that is a fake `id` on `PATH`: `cap_refuse_root` calls `id -u` rather than
reading `$EUID` specifically so a test can, a deliberate seam. Windows has no
root; it has SYSTEM, `S-1-5-18` and an Administrator role, and how you pretend
to be one is a property of the platform rather than of the capture.

The refusal must also **precede the step**. Exit 5 after running it is the whole
defect wearing the right exit code, so the step writes a marker and the marker
must be absent.

**7. Redaction is wired into the capture.**
Weaker than the redaction unit tests, on purpose, and asserting something
different: that masking is actually *on the capture path of this
implementation*. A correct redactor a second implementation forgets to call is
exactly the drift this suite is for, and a unit test of the function cannot see
it. Over-masking is asserted too.

The cases live in [`tests/fixtures/redaction-corpus.txt`](https://github.com/dbhq-uk/heliograph/blob/main/tests/fixtures/redaction-corpus.txt),
one file both implementations are measured against. Neither redactor is the
specification; that file is.

**8. A cancelled run keeps what it captured.**
A log that stops mid-sentence is still evidence, and usually the evidence you
wanted: the last line names the probe that was in flight.

The cancel has to be **proved**, and the proof is asked of the file rather than
of the driver: after the cancel the log must stop growing. Three seconds into a
ten-second step, a log that is merely unfinished looks exactly like a cancelled
one - a driver that cancelled nothing at all passed every other assertion here
until the suite started watching the size.

**9. The finished log reaches the far side.**
Read back from the receiving end, never from the working tree that wrote it.

## Property 9 is the one worth explaining

Properties 1 to 4 all assert things about the log **file**, and the file was
always correct. None of them asked whether it arrived.

It had not. On the relay and blob transports the finished log never left the
machine: the runner ended at a git push, unconditionally, so a station on any
other channel captured a perfect log and delivered nothing - no footer, no exit
code, no `RESULT`. It passed every other property throughout, and it was
invisible from the control side because the log was written correctly to local
disk every single time.

That is the argument for the property, and for the suite: **the properties you
have are the defects you catch.**

## Every transport, not just git

Property 9 answered that question on git alone for as long as it existed, which
made it a much weaker property than it looked. Properties 1 to 8 look at the
captured file, and the file is written correctly to local disk on every
transport - so a `tp_put_log` that returned success and did nothing whatsoever
would have passed the entire suite.

The whole suite now runs once per transport:

| | |
|---|---|
| **git** | a bare remote, and the log is read back out of it |
| **share** | a directory, and the log is read back from the share rather than the working tree |
| **relay** | a stub relay in memory, two keypairs, and the log is **unsealed with the control side's identity** - so a log sealed for somebody else, or signed by nobody, is not counted as delivered |

The relay stub is a queue with an HTTP interface and the relay's token rules,
which is the entire contract the station side depends on. It is deliberately
ignorant of the payload: every body is an opaque sealed envelope, stored and
handed back byte for byte. A stub that could read the messages would be one
that could accept an envelope the real relay would mangle.

Its **token scopes are asymmetric**, because the real ones are: a station token
may collect a request and publish status and logs, and may not queue a request
even for itself. One token for everything would let a station that used the
wrong credential pass here and be refused by a real relay. The suite asserts
that boundary against the stub rather than assuming it - a double more
permissive than the thing it stands in for is worse than no double.

And running it three times only proves three passes, so each transport is also
checked for **teeth**: `tp_put_log` is replaced with `return 0` - a delivery
that claims success and does nothing, the exact shape of the original defect -
and property 9 must fail. If it does not, it is reading the local file again.

**blob is not conformance-tested.** Its far side is an Azure storage account
and there is no honest way to stand one up offline. That exclusion is asserted
rather than assumed: the suite lists every transport in the toolkit and fails if
one is on neither the covered list nor the excluded one, so a transport added
later cannot go unnoticed.

## Two honest limits

**A driver skips loudly rather than passing.** A driver that cannot observe the
far side skips property 9 and says so. A scrollback reading as though
everything was checked when half was not is the same defect as a silently
skipped checksum.

**Nothing here exercises a capture against a real remote machine.** The
behaviour that matters is what a log looks like after a round trip through
someone else's terminal, and no test asserts that.

**Property 2 has an upper bound, and an overloaded runner can exceed it.** A 3
second gap in the source must appear as a 3 to 8 second gap in the log. The
lower bound is the property; the upper one guards against stamps applied at
flush rather than at read, and a badly stalled CI worker could produce a
legitimate gap larger than 8 seconds. It has not yet.

## Adding an implementation

A second implementation of the capture is permitted **only while it passes this
suite**. That is the whole rule, and it replaced a blanket prohibition: the
argument against a port was always about *untested* drift, and the suite makes
it testable.

Write a driver implementing `drv_name`, `drv_supports`, `drv_capture`,
`drv_bootstrap`, `drv_step`, `drv_capture_bg`, `drv_cancel`, `drv_deliver` and
`drv_delivered`, plus `drv_step_privileged` if the platform can simulate one
and `drv_teardown` if the far side is a process. Then make the suite pass
without touching the suite.

Nothing platform-specific belongs in the suite. Two things were in it and are
not now: the fake `id`, and the `sed -u` probe that decides whether a cancel can
keep anything. That probe is a fact about busybox, not about the property, and
it moved into `drv_supports cancel` where the implementation can state it.
