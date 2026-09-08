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
| **launchd** | macOS. A LaunchAgent, loaded for real |
| **`setsid` + `nohup`** | the fallback where neither exists. Honest about being one |

`HELIOGRAPH_SERVICE_NAME` overrides the unit name. One transport repo per
investigation is ordinary, and a fixed name would let the second install
silently replace the first.

## Windows: `service.ps1`

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
