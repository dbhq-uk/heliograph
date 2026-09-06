# CLI reference

Every command the control side has, and the reasoning behind the ones that are
not obvious.

```
heliograph init <estate> --dir <path> [--transport git|share|bundle] [--scope <name>]
heliograph estates
heliograph plant [--service] [--script]
heliograph send <step> [KEY=VALUE ...] [--note <text>]
heliograph watch [--interval 10s] [--timeout 0]
heliograph status
heliograph logs [--last] [<name>] [--gaps] [--min 10s]
heliograph doctor
heliograph version
```

Every command takes `-e` / `--estate`. With exactly one configured, it is
optional. With several it is required: sending a request to the wrong estate
runs a command on the wrong machine, and that is not recoverable by apologising.

## init

Remembers a transport by name, so every later command can be typed without a
path. It **attaches before saving**: an estate that names a directory which is
not a usable transport is worse than no estate at all, because it fails later,
from a command that had every reason to expect it to work.

## plant

Prints the message to send the operator. Generated rather than typed, because
every retyping of a clone URL is a chance to get it wrong on a machine nobody
can check afterwards.

It carries **no credential**, deliberately. A token pasted into a chat window is
in that history forever, and the operator usually already has one.

It also says what the loop will *not* do: refuses root, runs nothing that
changes state without `--allow-actions`, holds no credentials of its own. That
is not padding. Somebody is being asked to run a stranger's script on a
production machine, and the honest answer to "what does this do" is what gets it
approved.

## send

Publishes a request. The `id` is the trigger and nothing else is: documentation
and step edits land on a branch constantly, and if any change fired a run the
station would run on all of them.

Everything after the step name is environment, passed verbatim:

```bash
heliograph send net-probe HOSTS="sql01 sql02" PORTS=1433
```

Values containing spaces are re-quoted on the way out. The shell that invoked
the CLI has already eaten your quotes, and the station splits that line the way
a shell would, so an unquoted value would set the first word and try to *run*
the rest.

## logs --gaps

The reason the binary is worth installing.

```
$ heliograph logs --last --gaps
net-probe-20260906T091400Z.txt
412 captured lines

2 interval(s) of 10s or more, longest first.
Each is attributed to the line BEFORE it, which is what was running.

   3m12s  after  09:14:02 | Refreshing state...
     45s  after  09:17:14 | ---------- openssl s_client ----------
```

A log where **every line carries the same timestamp** is reported as an error
rather than "no gaps found". That log is a buffered capture, it reads perfectly,
and calling it clean would be the exact opposite of true.

## doctor

Answers "will this work from here" and changes nothing. Every line that reports
a problem also says what to do about it: a preflight line that names a fault
without a remedy is a defect, because the person reading it usually cannot ask
anybody.
