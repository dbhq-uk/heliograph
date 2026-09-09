# Where a station can run

Sometimes there is no willing human to start `./start.sh` and leave it running.
A station needs very little, so it can run almost anywhere - and this page
publishes the contract first, then says honestly which hosts have actually been
proven.

## The host contract

A station needs five things and nothing else:

1. a process that can run bash
2. reach to one transport, **outbound only**
3. a restart policy
4. a non-root account
5. somewhere to write a file it then hands off

No VNet, no storage account, no inbound port, no persistent disk. **Git or the
relay is the persistence**: if the compute dies, you run the step again. The
checkout is transient everywhere.

Anything meeting those five is a viable host, whether or not it appears below.

## Every host, and what it runs

The distinction between proven and written is kept deliberately. Shipping
twenty untested templates would spend the credibility of the ones that work.

| host | what starts the loop | status |
|---|---|---|
| operator's terminal | `./start.sh` | **proven** - 145 assertions, every CI run |
| Docker | `entrypoint.sh`, then `exec ./start.sh` | **proven** - CI builds the image and runs a loop in it |
| Kubernetes | the same image, one replica | **proven** - CI applies the manifest to a real cluster |
| systemd `--user` + lingering | `service.sh install` | **proven** - CI installs a unit and finds a running loop |
| launchd | `service.sh install` | **proven** - CI loads a real LaunchAgent on macOS |
| setsid + nohup | `service.sh install`, where neither exists | **proven** |
| Windows scheduled task | `service.ps1 install`, then `station.ps1` | **proven** - CI registers and reads back the task |
| GitHub Actions | `./start.sh -- --once` | written |
| Azure Pipelines | `./start.sh -- --once` | written |
| Azure Container Instances | the image | **deployed live**, then torn down |
| Azure Web App for Containers | the image | **deployed live**, then torn down |
| Azure Container Apps Job | the image, on a schedule | **deployed live**, then torn down |
| Azure VM | `cloud-init.sh`, then a systemd unit | **deployed live** on `Standard_D2s_v3` in westeurope |
| Azure Function App | `pigeonhole.sh`, on a timer | written and validated, **never deployed** |
| ECS Fargate, Cloud Run, anything else | your own, against the contract above | recipes, not templates |

## Which transport works on which host

`start.sh` asks the transport rather than assuming git. Set `TRANSPORT` and that
transport's variables and the preflight checks *that* channel - so a relay or
blob station now starts with the same command, and gets the same table.

Most hosts now carry those variables too. The container image plants the station
payload itself, so a transport with nothing to clone has one; the service
installers read an env file, because a detached process inherits nothing from
the shell that installed it.

| host | git | Azure Blob | relay | file share | bundle, object store |
|---|---|---|---|---|---|
| operator's terminal | yes | **yes** | **yes** | **yes** | no station side |
| Docker, Kubernetes | yes | **yes** | **yes** | **yes** | no station side |
| systemd, launchd, setsid | yes | **yes** | **yes** | **yes** | no station side |
| Windows scheduled task | yes | not plumbed | not plumbed | not plumbed | no station side |
| pipelines | yes | not plumbed | not plumbed | not plumbed | no station side |
| Azure ACI, Web App, Apps Job, VM | yes | not plumbed | not plumbed | not plumbed | no station side |
| Azure Function App | no `git` in the image | **yes** | not plumbed | not plumbed | no station side |

**"yes"** means: export `TRANSPORT` and the transport's variables, then run
`./start.sh` exactly as you would for git. The relay also needs
`heliograph-seal` present and a key exchange completed, and `./start.sh --check`
says so if either is missing.

**The relay in a container needs no extra step.** It is the one transport that
needs a binary - `heliograph-seal`, argued for in [the relay page](/relay) - and
the image carries it, built from the same commit as the payload. The entrypoint
points the station at it and says which checksum it used. Only the two key files
have to be mounted, because those are yours.

**"not plumbed"** means the station can do it and the host recipe cannot carry
it there. The Azure templates, the pipeline definitions and `service.ps1` still
set up a git checkout and pass no `TRANSPORT` through, so selecting another
transport there means editing the recipe. Nothing refuses it - it just is not
wired yet.

**"no station side"** means the CLI implements the transport and the station
has no code to read it, so the combination cannot work at all.

What each still needs is in
[the roadmap](https://github.com/dbhq-uk/heliograph/blob/main/docs/plans/2026-09-08-powershell-and-docs-roadmap.md).

### In a container

The image carries the station payload, planted at build time by the same
`bootstrap.sh` that plants a transport repo. So there is nothing to clone and
nothing to mount:

```bash
docker run --rm \
  -e TRANSPORT=share -e SHARE_DIR=/mnt/ops -e SHARE_SCOPE=dns-timeouts \
  -v /mnt/ops:/mnt/ops \
  ghcr.io/dbhq-uk/heliograph-toolkit:latest
```

`REPO_URL` alongside a non-git `TRANSPORT` is **refused**, not ignored: it means
somebody believes the container is going to clone something, and it is not.

The git path is untouched. It still clones, and that clone is the one
authoritative copy of `start.sh` - the payload in the image is a fallback for
the transports that have no repository, not a second opinion about the ones
that do.

### As a service

A detached process inherits nothing from the shell that installed it. That has
always been true of `GIT_TOKEN`, and `service.sh` has warned about it for as
long as it has existed; for a relay or a share it is worse, because those have
no fallback file the way caplib reads `~/.git-token`.

So write the variables to `.station-env` beside the payload, mode 600:

```
TRANSPORT=relay
RELAY_URL=https://heliograph-relay.dbhq.uk
RELAY_ESTATE=payments
RELAY_STATION=db-a
RELAY_TOKEN=...
RELAY_IDENTITY=/home/ops/.heliograph-identity
RELAY_PEER=/home/ops/.heliograph-peer
```

`./service.sh install` reads it, tells you which variables that transport wants
if it is missing, and says so if the file is readable by anyone else. systemd
gets an `EnvironmentFile`; launchd and the `setsid` fallback source it before
`exec`, because a LaunchAgent plist is world-readable and a token has no
business being in one.

## Picking one

**A person's terminal is still the best host** when there is a willing person.
It needs no infrastructure request, and `./start.sh` prints its own preflight
to somebody who can read it. Reach past it when nobody will sit there.

| you want | reach for |
|---|---|
| the simplest thing that survives a logout | [systemd or a scheduled task](/service) |
| an image, and no OS to own | [Docker or Kubernetes](/containers) |
| cloud compute, no VM to patch | [Azure ACI or Container Apps Job](/azure) |
| a shell to debug the station itself | Azure Web App for Containers, or a VM |
| nowhere to keep a process at all | [Azure Function App](/azure) - a timer, not a loop |

## Two things that will waste your time

**The published image tag has no `v`.** Git tag `v1.0.0-rc1` publishes
`ghcr.io/dbhq-uk/heliograph-toolkit:1.0.0-rc1`.

**A GitHub transport repo needs `GIT_TOKEN_USER=x-access-token`**, or git
reports a missing username rather than a wrong one, which sends you looking at
the token.

## Running it in a pipeline

A build agent is a host too, and often the only compute in an estate that can
already reach both the git host and the target. `station/bash/pipelines/` ships
GitHub Actions and Azure Pipelines definitions. See [pipelines](/pipelines) -
including the loop guard that stops a log push re-triggering the pipeline that
produced it.
