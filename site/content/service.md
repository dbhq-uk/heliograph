# Making the loop outlive the session

The operator starts the station and walks away. If it dies when they log out,
they have not walked away - they have to come back, which is the relaying this
tool exists to remove.

## Why a plain `nohup` is not enough

`station.sh` deliberately does **not** trap `HUP`. Its cleanup signals the
running step's process group, so trapping `HUP` would kill an in-flight step
every time a connection dropped - which on a jump host is often.

So the answer is to start it somewhere the session cannot take with it.

## Linux and macOS: `service.sh`

```bash
./service.sh install                    # survive logout and reboot
./service.sh install --branch task/foo  # on a task branch
./service.sh install -- --once          # args after -- go to station.sh
./service.sh status
./service.sh logs
./service.sh stop
./service.sh uninstall
```

It picks the best mechanism available, in this order:

| | |
|---|---|
| **systemd `--user`, plus lingering** | the right answer on any modern Linux. `loginctl enable-linger` is what makes a user service survive logout |
| **launchd** | macOS. A LaunchAgent, loaded for real. Needs a bash 4 or newer - see below |
| **`setsid` + `nohup`** | the fallback where neither exists. Honest about being one |

`HELIOGRAPH_SERVICE_NAME` overrides the unit name. One transport repo per
investigation is ordinary, and a fixed name would let the second install
silently replace the first.

### On macOS, the LaunchAgent does not inherit your PATH

It gets `/usr/bin:/bin:/usr/sbin:/sbin`, and the only bash there is the 3.2 that
macOS still ships. The station's preflight refuses 3.2, so a plist that simply
says `/bin/bash` produces a station that fails on every start and is restarted
for ever by a `KeepAlive` doing exactly what it should. Every symptom points at
launchd, and launchd is innocent.

`service.sh install` therefore resolves an **absolute** path to a bash 4 or
newer at install time, writes that into `ProgramArguments`, and prepends its
directory to the LaunchAgent's PATH. With no such bash on the machine it
refuses to install rather than leaving a crash loop behind:

```
error no bash 4 or newer on this Mac, and /bin/bash is 3.2.
```

`brew install bash` and run it again.

## Windows, with Git for Windows: `service.ps1`

```powershell
.\service.ps1 install
.\service.ps1 install --branch task/foo
.\service.ps1 status
.\service.ps1 logs
.\service.ps1 stop
.\service.ps1 uninstall
```

A **scheduled task**, not a service. A Windows service needs installation
rights and a wrapper for a script; a scheduled task needs neither, runs as the
operator, and can be told to run whether that operator is logged on or not -
which is the entire requirement.

It registers the task against `station.ps1` rather than against a bash path, so
the Git-for-Windows discovery stays in one place. See [Windows](/windows).

Three settings that are not incidental:

- **`ExecutionTimeLimit` is zero.** The Windows default is three days, after
  which the task is stopped - and a loop that quietly stops after three days is
  precisely the failure this exists to prevent
- **`AtStartup`**, so it comes back after a reboot with nobody logged in
- **S4U logon**, which runs the task whether the operator is signed in or not
  and stores no password. The trade is that it gets no network *credentials*,
  which does not matter here: git authenticates with a token from a file or an
  ssh key, not with the Windows identity

## Windows, without bash: the PowerShell payload's own `service.ps1`

```powershell
.\service.ps1 install              # survive logout and reboot
.\service.ps1 install -- --once    # args after -- go to the loop
.\service.ps1 status
.\service.ps1 logs
.\service.ps1 stop
.\service.ps1 uninstall
```

A different file from the one above, and not interchangeable with it. That one
registers the **launcher** and refuses to install without a `start.sh` beside
it, so on a payload with no bash it refuses every time. Until this shipped,
`--flavour powershell` planted no way to survive a logout at all.

It registers `start.ps1` rather than the loop directly, so the **preflight runs
on every start**. A machine that has since had Constrained Language Mode
applied, or lost its share mount, refuses and says why instead of starting a
loop that cannot work.

### The hard part is not the task

**A scheduled task starts with a fresh environment and inherits nothing from
the shell that registered it.** So the transport's variables and any credential
simply are not there. The loop then starts, polls happily, captures a perfect
log and cannot deliver it - and the far side waits for hours with nothing
reporting a fault.

`service.ps1 install` therefore copies what the station needs into
`.station-env-ps` beside the payload, and the loop reads it at startup:

- **`KEY=value`, one per line, read and never executed.** A `.ps1` there would
  be a file the loop runs at every start, sitting in a directory the far side
  can write to on some transports. Configuration must not be code.
- **Values are verbatim** - everything after the first `=`. No quoting scheme,
  so a token containing a quote, a space or a backslash survives. A newline is
  *refused* rather than stripped, because it would forge a second variable and
  silently changing a credential is worse than not writing it.
- **A named list, not the whole environment.** Copying everything would put
  `PATH` and `TEMP` into a file that also holds a token, and make "what is this
  station configured with" unanswerable.
- **The environment wins.** Running the station by hand overrides whatever the
  service was installed with, so debugging does not start with editing a
  dotfile. A task has a clean environment, so there the file always applies.
- **It holds a token**, so its ACL is set to this account only - inheritance
  off, inherited rules dropped - which is the Windows equivalent of the bash
  side's mode 600. `uninstall` deletes it; leaving a credential behind is not
  tidying up.

`install` **refuses** when a detached loop could not deliver - no `SHARE_DIR`,
no origin remote, no credential at all - because that is the failure nobody
sees until hours later. `-Force` overrides it.

### Exit 75 is why the restart policy matters more here

A PowerShell station that updates itself **exits 75** rather than
re-executing: PowerShell has no `exec`, and a respawn is killed by the Job
Object the cancel depends on. The task is registered with `-RestartCount 5
-RestartInterval 1 minute`, so that exit is what makes the update take effect.
Without it, a self-update stops the station and looks exactly like a station
that finished.

`service.ps1 status` says so when it sees that code, because a non-zero result
otherwise reads as a fault.

## The credential is where an unattended loop actually fails

A detached process inherits nothing from the shell that started it.

```
GIT_TOKEN typed before ./station.sh      reaches the station
GIT_TOKEN typed before ./service.sh install   does NOT reach the service
```

The loop then starts, polls happily, and cannot deliver a single log - which is
discovered hours later by somebody waiting on the far side.

Both installers check for this and refuse rather than installing a loop that
cannot deliver. Write the credential to a file the detached process can read:

```bash
printf '%s' "$GIT_TOKEN" > ~/.git-token && chmod 600 ~/.git-token
```

```powershell
$env:GIT_TOKEN | Out-File -NoNewline -Encoding ascii "$HOME\.git-token"
```

The capture library reads `~/.git-token` already, so nothing else has to
change. `-Force` overrides the refusal if you know better.

A forwarded ssh agent key is the other common trap: it dies with the session,
which is exactly when an unattended loop needs it. Use a key on disk, a deploy
key, or a token.

## Checking it is really running

```bash
./service.sh status     # what it is doing
./service.sh logs       # follow it
```

On Windows, `LastTaskResult` of **267009** means "currently running", not an
error code. It reads like one to anybody who has not looked it up, so
`status` says so.
