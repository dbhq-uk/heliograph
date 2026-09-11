# Intercom - when you can reach the station

Every other transport exists because you cannot reach the far side. This one is
for the narrower case where you *can*: the station sits behind a public HTTPS
endpoint, and storage or git in the middle is indirection with no purpose.

`./intercom.sh` in the transport repo submits a step over HTTPS and gets the
log back in seconds rather than in poll intervals.

```bash
./intercom.sh run steps/net-probe.sh HOSTS="sql01 sql02"
```

## When this is worth reaching for

Realistically, one shape: an **Azure Function App** inside the VNet, which has
a public HTTPS front door *and* sits inside the network. That combination is
unusual and is exactly what makes intercom worth having when it occurs.

If you cannot reach the station, use [any other transport](/transports). If you
can reach the *machine* properly, use SSH and do not use heliograph at all.

## Read this before exposing it

Intercom makes a trade the other transports do not, and it is stated here
rather than buried.

**It ships the script it wants run.** So `# heliograph-mode: read-only`
degrades from a *control* into a *claim the caller makes about their own file*.
The runner still reads the declaration out of the file it is about to execute,
and a submitted script with no header refuses exactly as a committed one does -
but the caller wrote both the script and the header.

The gates that remain are therefore the ones around the endpoint:

- **The function key.** Anyone holding it can submit a script. Treat it as what
  it is: remote code execution on that host, as that account
- **The IP allowlist.** Not optional in any estate that would care
- **The account the station runs as**, which is the blast radius as always

**Do not expose this endpoint publicly and rely on the mode header.** That is
not what it is for and it will not hold.

## The blob alternative, for when you cannot reach it

The same Azure Function host also carries the Azure Blob transport, and the
estate picks. `./drop.sh` is the control side of that one:

```bash
./drop.sh send <id> <step>     # queue a request into the lane
./drop.sh watch <id>           # follow it
./drop.sh bundle               # upload the station's own code
```

Neither side ever reaches the other: both dial storage. It works in a subnet
with no route off it at all, which is the case it was built for. The station
side is `pigeonhole.sh`, which is the loop for a host invoked fresh on a timer
rather than left running.

**Measure before reaching for either.** Git is better when git works, and an
image pull succeeding proves nothing: a container platform pulls on its own
side, so a container can start cleanly on a host with no network at all.

## Both ship in one host

Leaving `HELIOGRAPH_ACCOUNT` unset leaves intercom off. Setting
`HELIOGRAPH_SCHEDULE` to a date that never comes leaves the blob timer off.
Running both is fine and is what the reference deployment does. See
[Azure](/azure).

## Where the station payload is

`intercom.py` shells out to `run.sh` rather than reimplementing the capture -
one implementation of the capture, and `run.sh` owns the mode gate as well. It
finds the payload by looking for `run.sh` and `caplib.sh` beside itself and then
upwards, which covers both shapes it ships in: flattened in the deployment
package, and nested under `azure/function/` in a checkout.

`HELIOGRAPH_TOOLKIT` overrides that with an explicit path, for a deployment that
mounts the payload somewhere else. It is read from the **environment only, never
from a request**: it chooses which `run.sh` executes, so a request able to set it
would be a request able to choose the code. A path that has no `run.sh` and
`caplib.sh` in it is refused rather than silently falling back to the search.
