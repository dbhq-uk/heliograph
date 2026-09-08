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

## Proven, versus written

The distinction is kept deliberately, because shipping twenty untested
templates would spend the credibility of the ones that work.

| host | status |
|---|---|
| Docker | proven - the image is built and published by CI |
| Kubernetes | proven - CI applies the manifest to a real cluster |
| systemd (`--user` + lingering) | proven |
| setsid + nohup fallback | proven |
| Windows scheduled task | proven on a real Windows runner in CI |
| launchd (macOS) | proven - CI loads a real LaunchAgent on a macOS runner |
| Azure Container Instances | **deployed for real**, then torn down |
| Azure Web App for Containers | **deployed for real**, then torn down |
| Azure Container Apps Job | **deployed for real**, then torn down |
| Azure VM + cloud-init | **deployed for real** on `Standard_D2s_v3` in westeurope: booted, cloned, ran a step, log came back |
| Azure Function App (Flex) | written and validated; **never deployed** |
| ECS Fargate, Cloud Run, everything else | recipes against the contract, not owned as templates |

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
