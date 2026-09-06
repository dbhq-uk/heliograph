# Quick start

From nothing to a captured log, in five steps. It assumes somebody on the far
side can run one command once, and nothing else.

## 1. Make a transport repo

Private, its own repo, nothing else in it.

**It must be private, and it must be separate.** Captured logs are committed to
it, so everything the far side prints lands in that history permanently.

```bash
git clone https://github.com/dbhq-uk/heliograph-skill
./heliograph-skill/skills/heliograph/scripts/bootstrap.sh ~/transport/payments
cd ~/transport/payments && git init && git remote add origin <private-url> && git push -u origin HEAD
```

## 2. Tell the CLI about it

```bash
heliograph init payments --dir ~/transport/payments
heliograph doctor
```

`doctor` proves read access, write access and the credential in force before a
long run captures evidence it then cannot deliver. Read access is not write
access, and learning the difference afterwards costs a whole round trip.

## 3. Send the operator one command

```bash
heliograph plant
```

That prints the message to send them: what to run, what it does, and what it
will **not** do. They run it once and walk away. After that the loop is the
transport in both directions, and nobody has to relay anything.

## 4. Ask a question

```bash
heliograph send env                              # what is this box, really
heliograph send net-probe HOSTS="sql01 sql02"    # can it reach those, on which ports
heliograph watch                                 # follow it
```

Start with `env`, whatever the investigation turns out to be about. A prior
finding is a hypothesis to re-test, never a premise to build on.

## 5. Read the log

```bash
heliograph logs --last            # the whole thing
heliograph logs --last --gaps     # where it stalled
```

Scan the timestamp column before reading the content. `--gaps` does it for you:

```
   3m12s  after  09:14:02 | Refreshing state...
```

The gap belongs to the line **before** it. The stamp on a line is when that line
was produced, so a long interval means the operation named on the preceding line
is what took the time.

**A green exit means the probes that ran passed, not that the work happened.**
Verify the outcome, not the exit code.
