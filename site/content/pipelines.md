# Running a station in a pipeline

A build agent is a host too, and often the only compute in an estate that can
already reach both the git host and the target. `station/bash/pipelines/` ships
a GitHub Actions workflow and an Azure Pipelines definition.

## When this is the right answer

- The estate has a build agent inside the network and nothing else you can run on
- Getting a VM approved is a change request; adding a pipeline is not
- You want the run to be someone's audited, logged CI job rather than a process
  on a laptop

## When it is not

A pipeline job has a **time limit**, and a station is a loop. Run it with
`--once` so it answers one request and exits, and accept that the loop is
"whenever the pipeline runs" rather than every five seconds.

If you need a genuine loop, use [a real host](/hosts).

## The loop guard, which is not optional

The station delivers its log by committing to the transport repo. If the
pipeline is triggered by pushes to that same repo, the log push re-triggers the
pipeline, which runs a step, which pushes a log, which re-triggers it.

Every commit the station makes carries **`***NO_CI***`** in its message.

- **GitHub Actions** refuses to trigger a workflow on a push made with
  `GITHUB_TOKEN`, so it needs no help - but the marker travels anyway
- **Azure DevOps** has no equivalent, so the marker in the commit message is
  the only guard, and it is the one that travels with the commit rather than
  living in one host's trigger configuration

If you write your own pipeline, honour it. A runaway loop on somebody else's
build minutes is a bad first impression.

## The credential

The agent's own git credential is usually already there and usually enough. If
not, the same rules as everywhere else apply: `GIT_TOKEN` or `GIT_TOKEN_FILE`,
and `GIT_TOKEN_USER=x-access-token` for GitHub.

Use the pipeline's secret store. Do not put it in the YAML.

## Do not run it as root

Most container-based agents run as root by default, and the runners refuse
that: this tooling holds no credentials of its own, so the account it runs as
is the whole blast radius, and as root that is the machine.

Add a non-root user in the job, or set `ALLOW_ROOT=1` if the agent image
genuinely has no other and you accept what that means. See
[security](/security).
