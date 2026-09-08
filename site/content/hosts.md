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
| operator's terminal | `./start.sh` | **proven** - 94 assertions, every CI run |
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

**Git, everywhere, and almost nothing else yet.** The reason is one line: every
host above except the Azure Function App starts the loop through `start.sh`,
and `start.sh` refuses to continue without a git `origin` remote.

| host | git | Azure Blob | relay | share, bundle, object store |
|---|---|---|---|---|
| operator's terminal | yes | by hand | by hand | no station side |
| Docker, Kubernetes | yes | no | no | no station side |
| systemd, launchd, setsid | yes | not as a service | not as a service | no station side |
| Windows scheduled task | yes | not as a service | not as a service | no station side |
| pipelines | yes | no | no | no station side |
| Azure ACI, Web App, Apps Job, VM | yes | no | no | no station side |
| Azure Function App | no `git` in the image | **yes** | no | no station side |

**"by hand"** means: skip `start.sh`, set `TRANSPORT` and the transport's
variables yourself, and run `./station.sh` directly. Nothing configures it for
you, and the relay also needs `heliograph-seal` planted.

**"not as a service"** is the same thing with a sting. `service.sh` and
`service.ps1` both start the loop through `start.sh`, so running a non-git
transport under them means bypassing the very mechanism you installed - and
with it the restart policy, which is the reason to use a service at all.

**"no station side"** means the CLI implements the transport and the station
has no code to read it, so the combination cannot work at all.

What each still needs is in
[the roadmap](https://github.com/dbhq-uk/heliograph/blob/main/docs/plans/2026-09-08-powershell-and-docs-roadmap.md).

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
