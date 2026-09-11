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

## The PowerShell station has no service installer yet

`service.ps1` ships in the **bash** payload and registers the task against
`station/bash/station.ps1`, the launcher - so it requires `start.sh` beside it
and refuses to install without one. The [pure PowerShell
station](/windows#the-powershell-station-for-a-box-with-no-bash) has no
equivalent, and `--flavour powershell` plants no service installer at all.

So on a box with no bash, keeping the loop alive after a logout is currently
something you arrange yourself. A scheduled task that works today:

The settings below are the ones `service.ps1` itself registers, which CI proves
on a real Windows runner - only the command it runs differs.

```powershell
$payload = 'C:\ops\payments'
# Set-Location first: the station resolves its payload from its own path, but
# the transport's variables and any relative LOG_DIR come from the working
# directory.
$inner = "Set-Location '$payload'; .\station.ps1"
$action = New-ScheduledTaskAction -Execute 'powershell.exe' `
            -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command `"$inner`""
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" `
            -LogonType S4U -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet `
            -ExecutionTimeLimit ([TimeSpan]::Zero) `
            -RestartCount 5 -RestartInterval (New-TimeSpan -Minutes 1) `
            -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
Register-ScheduledTask -TaskName heliograph -Action $action -Trigger $trigger `
            -Principal $principal -Settings $settings
```

`-RestartCount` and `-RestartInterval` are not optional here, and they matter
more than they do for the bash station. A PowerShell station that updates
itself **exits 75** rather than re-executing - PowerShell has no `exec`, and a
respawn is killed by the Job Object its own cancel depends on. Without a
restart policy, a self-update stops the station instead of replacing it.

**What this does not do, and the installer would.** It does not carry the
transport's variables into the task. A detached process inherits nothing from
your shell, which is the next section and is where an unattended loop actually
fails - so set them machine-wide, or add them to `$inner` before
`.\station.ps1`.

This is a gap rather than a decision, and it is recorded as the next thing to
build for that payload.

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
