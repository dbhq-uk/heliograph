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
Simulated rather than actually run as root. `cap_refuse_root` calls `id -u`
rather than reading `$EUID` specifically so a test can put a fake `id` on
`PATH` - a deliberate seam, not an accident.

**7. Redaction is wired into the capture.**
Weaker than the redaction unit tests, on purpose, and asserting something
different: that masking is actually *on the capture path of this
implementation*. A correct redactor a second implementation forgets to call is
exactly the drift this suite is for, and a unit test of the function cannot see
it. Over-masking is asserted too.

**8. A cancelled run keeps what it captured.**
A log that stops mid-sentence is still evidence, and usually the evidence you
wanted: the last line names the probe that was in flight.

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

## Two honest limits

**A driver skips loudly rather than passing.** A driver that cannot observe the
far side skips property 9 and says so. A scrollback reading as though
everything was checked when half was not is the same defect as a silently
skipped checksum.

**Nothing here exercises a capture against a real remote machine.** The
behaviour that matters is what a log looks like after a round trip through
someone else's terminal, and no test asserts that.

## Adding an implementation

A second implementation of the capture is permitted **only while it passes this
suite**. That is the whole rule, and it replaced a blanket prohibition: the
argument against a port was always about *untested* drift, and the suite makes
it testable.

Write a driver implementing `drv_name`, `drv_supports`, `drv_capture`,
`drv_bootstrap`, `drv_step`, `drv_capture_bg`, `drv_deliver` and
`drv_delivered`. Then make the suite pass without touching the suite.
