# New transports and new stations, surveyed

**Date:** 2026-09-10
**Status:** draft, for review

A survey of what else could carry the loop, what else could run it, and what
else could drive it. Nothing here is a commitment. It exists so that the next
person who asks *"could it go over X"*, *"could it run on Y"* or *"could I
drive it from Z"* finds an answer with a reason attached rather than starting
the same search again.

**The evidence is a separate file.** Measurements, vendor documentation and
every source are in
[`../research/2026-09-10-transports-hosts-and-control-nodes.md`](../research/2026-09-10-transports-hosts-and-control-nodes.md),
so a verdict here can be re-argued against what was actually measured. This
file holds the verdicts and nothing else.

Two contracts already decide most of it, and both are published:

| | |
|---|---|
| a transport | ten station-side functions, seven of them on the capture path, and `tp_check` proves a **write**. See [transports](https://heliograph.dbhq.uk/transports) |
| a host | a process that can run bash, outbound reach to one transport, a restart policy, a non-root account, and somewhere to write a file. See [hosts](https://heliograph.dbhq.uk/hosts) |

A candidate that fails either is not a near miss, it is a different product.

## Triage

Every candidate, with a verdict. **Do** is next or near-free. **Later** has real
value and waits on something named. **Maybe** is revisited when an engagement
asks for it. **Never** is decided, and the reason travels with it so it is not
re-proposed.

### Transports

| | | |
|---|---|---|
| artifact repository - Artifactory, Nexus raw | **do** | largest population, `blob.sh` is the template |
| Google Cloud Storage | **do** | may already work through the object store. One CI job to find out |
| OCI registry, distribution API | later | after the artifact repository, which proves the same HTTP shape |
| Microsoft Graph - SharePoint, OneDrive | later | the only reach into an M365-only estate. Needs OAuth refresh, which nothing here has |
| ServiceNow or Jira attachments | maybe | the audit record is the right home for a log. A ticket is not a queue, and anyone who can attach can queue |
| AMQP, MQTT, Kafka | maybe | parked by the master design, not rejected. No pure-bash client, so it is the station's first dependency |
| animated QR, screen to camera | maybe | the one channel left when removable media is banned and a screen is the only way out - a Citrix session with the clipboard off. QR v40 holds 2953 binary bytes a frame, so it is three orders of magnitude faster than sound. **Asymmetric**: a request is a few hundred bytes and fits in one static frame, while a log needs the station to render animated frames, which is a package and a display it does not have |
| data over sound - ggwave and similar | never | **8 to 16 bytes a second.** A 50KB log is 53 minutes of clean audio. It is a C library needing an audio backend, so it breaks *nothing is installed on the far side*. No container, Kubernetes pod, ACI group or VM in the host table has a speaker or a microphone, so the population is zero. And ultrasonic egress from an isolated machine is a documented air-gap attack technique, which is the DNS argument again |
| LoRa and Meshtastic | never | 84 bytes a second on the default preset, 2.7KB/s at the fastest, before headers, hops and a 10% hourly duty cycle in the EU. It also needs a radio, and in most estates an unauthorised transmitter is a larger incident than an unauthorised network connection |
| rclone, Syncthing, croc, magic-wormhole | never | each is a binary or a daemon on the far side, which is the one thing this refuses. croc and magic-wormhole are also the relay we already have but worse: both ends must be online at the same moment, and heliograph is store-and-forward on purpose. Worth one line in the docs instead - an operator who already runs rclone has a file share or an object store, and the existing transports work through it without heliograph knowing |
| WebDAV | never | correct protocol, no population that has it and not one of the five above |
| Vault, SSM Parameter Store, App Configuration | never | a log does not fit in a parameter, and chunking one across secrets is a transport built to be abused |
| email | never | declined by the master design: large operational burden, small population |
| chat - Slack, Teams | never | captured logs in a chat system is a compliance problem, not a feature |
| DNS | never | a covert channel gets the product banned from the estates it targets |
| raw TCP, reverse tunnel | **superseded** | An unauthenticated always-on reverse connection stays refused, and for the original reason. The **beam** is the answer that was designed instead: off unless explicitly enabled on both ends, sealed, signed, and torn down when idle. See the beam design (S4) and the direct beam (S6) |

### Stations

| | | |
|---|---|---|
| AWS ECS Fargate | **do**, with an account | maps onto the ACI template. The plainest gap in the product, and issue #5 already ruled that it stays a recipe until somebody can deploy it |
| AWS EC2, user data | **do**, with an account | maps onto the VM template, and `cloud-init.sh` exists. Same condition as Fargate |
| GitLab CI | **do** | one YAML file against a pattern that already exists |
| Kubernetes CronJob | **do** | the same image with `--once`, beside the Deployment |
| Arista EOS | **do** | documented, supported bash. Gated on one curl-only transport landing |
| AWS Lambda, EventBridge schedule | later | the Function App's `pigeonhole.sh` pattern, ported |
| Cisco IOS-XE Guest Shell | later | bash 4 in an LXC bundled with the image. Needs a device to prove it on |
| NVIDIA Cumulus Linux | later | Debian. It is a server that forwards packets |
| GCP Cloud Run Job and GCE | later | after AWS, which is the larger gap |
| Ansible AWX | later | a `plant --via` route, not a host. It moves SSH to the control plane rather than removing it |
| F5 BIG-IP | maybe | bash beside tmsh. Real, and a narrower population than the switches |
| Jenkins | maybe | a third pipeline definition, once GitLab CI proves the pattern generalises |
| Termux on Android | maybe | a day's work, and it proves the host contract means what it says |
| Synology and QNAP NAS | maybe | Docker already covers it. A line in the docs, not a template |
| BusyBox devices - OpenWrt, ESXi shell | maybe | needs a third station in POSIX `sh`. Not before the PowerShell one is finished |
| z/OS UNIX System Services | maybe | bash exists. EBCDIC against a byte-for-byte capture contract is research, not a plan |
| IBM i PASE, AIX, Solaris | maybe | the softer landing: ASCII throughout, LF line endings the documented hazard |
| microcontrollers - ESP32 and similar | never | no process, no shell, no filesystem. A different product |
| iOS as a station | never | no unattended background execution, and no shell an estate would sanction |

### Control nodes

The near side has had no survey at all, and it is the half a person actually
touches. Two facts bound it, and both are already true in the code:

- **The binary already covers six targets.** `release.yml` builds
  linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 and
  windows/arm64, static, `CGO_ENABLED=0`
- **Only the git transport needs anything installed.**
  `internal/transport/git.go` shells out to `git`; relay, share, object store
  and bundle are pure Go. A control node with no git can still drive an estate

| | | |
|---|---|---|
| the near side without the CLI | **do** | the mirror of `station/bootstrap.sh`, and it already works: a request is a `key: value` text file, and `Marshal` is commented *"sed does not need them, but the operator reading the file does"*. A laptop that permits git and no new binaries is a control node today, undocumented |
| Android, through Termux | **do** | linux/arm64 is already built and git is a Termux package, so this is a CI job and a paragraph rather than a port. An on-call phone that can publish a step and read a log back is worth more than it sounds |
| ChromeOS | **do** | Crostini is Linux. One line, once somebody checks it |
| Claude Code on the web | **do** | a far better fit than remote MCP, because the sandbox is a real shell: the CLI is a static binary and it simply runs. Three real blockers, none of them ours to fix - the session proxy's allowlist, an estate credential living in somebody else's cloud, and a config directory wiped every session. Prove the one combination that can work and document the rest honestly |
| Homebrew, winget, scoop | later | not capability, adoption. `install.sh`, npm and `.mcpb` exist and these three are the gap |
| remote MCP, over Streamable HTTP | later | **the only route to a web or mobile control node**, and the reason is structural: Claude's custom connectors reach the server from Anthropic's cloud even on Desktop, so a connector needs a publicly reachable endpoint. That endpoint would hold the transport credential, and on the relay it would hold the signing identity too - which is precisely the *"a party able to forge a request has code execution inside every estate at once"* hazard the relay spec exists to refuse. It is buildable only as a **courier**, with signing left on a device holding the key. Design work, not plumbing |
| a CI job as the control node | later | send a step on a schedule and fail the build on what the log says, which is continuous verification of a machine nobody can log into. No new code, one example workflow, and a genuinely new use of the near side |
| FreeBSD | maybe | one line in the target list, and no evidence anybody wants it |
| iOS and iPadOS natively | never | no way to ship a binary or run git. Reachable only through remote MCP, so it is that row and not its own |
| a hosted web control plane | never | it must hold every estate's transport credential and signing key to be useful, which makes the operator of that service able to run commands inside every connected estate. That is the one thing the relay is architected not to be, and building it beside the relay would spend the claim |

## What the master design already decided

Restated so it is not re-proposed every six months. From
[`2026-09-06-heliograph-next-design.md`](2026-09-06-heliograph-next-design.md):

- **TCP is dropped.** A persistent reverse connection is a C2 channel by any
  blue team's definition, and not being one is why this class of tool is
  permitted at all
- **DNS is declined.** A covert channel gets the product banned from the
  estates it targets
- **Email is declined.** It genuinely works in mail-only air gaps, and it
  carries a large operational burden for a small population
- **Chat is declined.** Captured logs in a chat system is a compliance
  problem, not a feature
- **Message brokers are parked.** AMQP and MQTT have value where an estate
  already runs one, and no pure-bash client exists, so the station would gain
  its first dependency

Nothing found in this survey changes any of those five.

---

# Transports

## 1. Artifact repository: Artifactory and Nexus

**Recommended first.** It is the largest population of the lot and the
cheapest to build.

Every regulated estate that refuses a git host and refuses a storage account
still has an artifact repository, because that is how software gets into the
estate at all. It is already an approved egress path with an owner, a change
record and a credential model, which is the whole argument: the transport is
one nobody has to get approved.

Both are a plain HTTP file store to anything that can run curl:

```
Artifactory   PUT/GET  https://<host>/artifactory/<repo>/<path>
Nexus (raw)   PUT/GET  https://<host>/repository/<repo>/<path>
```

Auth is Basic with an access token, or a Bearer header. No SDK, no CLI, no jq.
It is `blob.sh` with different URLs and a different credential, which is 222
lines of precedent.

**The layout follows the object store's**, because the same shape has already
been argued once:

```
<repo>/<prefix>requests/<lane>.txt
<repo>/<prefix>status/<lane>.txt
<repo>/<prefix>logs/<name>.txt
```

**The open question is atomicity**, and it is the one the file share already
taught us to ask. A GET arriving during an in-flight PUT must not return a
half-written log, because a log that ends mid-line with no footer is the one
shape an operator reads as *still running*. JFrog documents checksum-based
storage and an atomic flag for exploded archives; neither says what a
concurrent reader sees. **Settle it with a round trip, not with a document.**

`tp_check` has an obvious cheapest write: PUT a `heliograph-write-check` file
into the lane and delete it. Deploy permission without delete permission is a
common configuration and would fail the delete, so the check should treat a
failed delete as a warning and a failed PUT as a refusal.

Sources: [JFrog cURL integration](https://docs.jfrog.com/integrations/docs/curl-integration),
[Nexus raw repositories](https://help.sonatype.com/en/raw-repositories.html).

## 2. OCI registry, over the distribution API

**Recommended second, and it is the genuinely novel one.**

An estate that runs containers permits registry traffic by definition, and a
registry has been a general artifact store since ORAS: an artifact is anything
a user needs to run your software, and it no longer has to be wrapped in an
image to be pushed.

It needs no new station dependency. The distribution API is three HTTP calls
and a sha256, and the station's preflight already requires a working sha256:

```
POST /v2/<name>/blobs/uploads/            start a session
PUT  <location>?digest=sha256:<hex>       the log, as a blob
PUT  /v2/<name>/manifests/<tag>           the manifest that names it
```

**Two things will cost time, and both are known in advance.** The first is the
`WWW-Authenticate` challenge and token exchange, which every registry does
slightly differently and which ORAS exists to hide. The second is that a
manifest is content-addressed, so a station that writes a status every sixty
seconds writes a new tag pointer every sixty seconds, and registry garbage
collection is a per-vendor question. `tp_put_progress` may have to be
declared unsupported through `tp_capabilities` rather than implemented badly.

**The real limit is permission, not protocol.** Many estates allow pull and
confine push to CI. That is a `tp_check` that fails honestly at start rather
than an hour later, which is the design working.

Sources: [ORAS](https://oras.land/docs/),
[ACR and OCI artifacts](https://learn.microsoft.com/en-us/azure/container-registry/container-registry-manage-artifact).

## 3. Microsoft Graph: SharePoint and OneDrive

**Recommended third.** Same shape as the artifact repository, larger
credential problem.

There is a real population here that none of the current transports reach: a
Windows estate whose only sanctioned egress is Microsoft 365. The file share
does not cover it, because the share transport needs a mount and this is an
API.

```
GET  /drives/{drive}/items/{item}/content                  read
PUT  /drives/{drive}/items/{parent}:/{name}:/content        write, under 4MB
POST /drives/{drive}/items/root:/{name}:/createUploadSession  write, over 4MB
```

**The 4MB boundary is not an edge case here, it is the normal case.** A
captured log crosses it easily, so the upload-session path is required rather
than optional, and that is a second code path with `Content-Range` headers on
a pre-authenticated URL that does **not** take the bearer token.

The credential is the harder half. Client credentials with a certificate is
the right shape for an unattended station; device code is the right shape for
the operator's first run and is a poor fit for something that has to survive a
reboot at 3am. Refresh handling is new work that no existing transport has.

Source: [upload small files](https://learn.microsoft.com/en-us/graph/api/driveitem-put-content?view=graph-rest-1.0),
[download driveItem content](https://learn.microsoft.com/en-us/graph/api/driveitem-get-content?view=graph-rest-1.0).

## 4. Google Cloud Storage, probably for free

`internal/transport/objstore.go` signs with SigV4 and GCS accepts SigV4 on its
S3-compatible XML API with HMAC keys. So this may already work with
`--dir https://storage.googleapis.com --region auto` and no code at all.

**Do not put it in the table until a round trip proves it.** The cost is one
CI job, and the outcome is either a documented supported store or a known
reason it is not.

## 5. The ticket, as a transport

**Interesting, not recommended yet.** Recorded because it will be proposed.

ServiceNow and Jira both expose attachments over plain REST
(`POST /api/now/attachment/file?table_name=...&table_sys_id=...`,
`GET /api/now/attachment/{sys_id}/file`), and in a change-controlled estate the
ticket is already where the operator is pasting half a terminal today. Putting
the captured log there puts it in the audit record rather than beside it, which
is the opposite of the chat argument the master design declined.

Two things stop it being a recommendation. **A ticket is not a queue**: polling
an incident for a new attachment is a cron loop with none of the latency the
git push gives, and attachments have no ordering guarantee. And **the blast
radius is wrong**: anyone who can attach a file to that ticket can queue a
request, and in most estates that is a much wider set of people than can write
to a repository.

Revisit if a real engagement asks for it, with the request side pinned to a
field only the change owner can write.

## Transports declined here

- **WebDAV.** Correct protocol, no population that has WebDAV and not one of
  the five above
- **Vault, SSM Parameter Store, App Configuration.** Value limits kill it. A
  log does not fit in a parameter, and chunking a log across secrets is a
  transport built to be abused
- **Kafka.** Same reason as AMQP and MQTT: no pure-bash client, so it is the
  station's first dependency

## The bandwidth gate, which is new

The exotic proposals - sound, radio, light, anything that leaves the building
without a network - all fail the same test, and it is worth stating once so it
can be applied without re-arguing.

**This tool's first rule is that a log is never truncated.** `ReadLog` returns
one log whole, and the comment on it says there is no line limit and there will
not be one, because *"the line somebody truncates is the line they needed"*.

So throughput is not a performance characteristic here, it is a gate. A channel
carrying 16 bytes a second turns a 50KB log into 53 minutes of transmission,
and everybody using it will start trimming the step's output to make it
bearable. That is the product being quietly dismantled by its own transport.

The rough floor is the size of a real captured log divided by a wait somebody
will actually tolerate. Call it tens of kilobytes a second. Sound (8-16 B/s)
and LoRa (84 B/s default) are three orders of magnitude below it. Animated QR
(2953 bytes a frame) is the only one of the three that clears it, which is why
it is the only one that is a maybe.

Two further tests each of these has to pass anyway, and most do not:

- **Does any host in the table have the hardware?** No container, pod, ACI
  group or VM has a speaker, a microphone, a radio or a camera
- **Is it a covert channel?** Ultrasonic and RF egress from an isolated machine
  are documented attack techniques. Shipping one gets the product banned from
  the estates it is built for, which is the DNS argument in different clothes

---

# Stations

## 1. The AWS host family

**Recommended first, and it is the plainest gap in the product.**

`station/bash/azure/` holds five templates and `station/bash/aws/` does not
exist. The site's own [`/compared`](https://heliograph.dbhq.uk/compared) page argues heliograph against
**AWS SSM Run Command**, so a reader arriving from that comparison finds a
persuasive argument and nothing to deploy.

Three, in order, and each maps onto something already written:

| | maps onto |
|---|---|
| **ECS Fargate task** | the ACI template. Same image, same env, same transport selection |
| **EC2 with user data** | the VM template, and `cloud-init.sh` already exists |
| **Lambda on an EventBridge schedule** | the Function App's `pigeonhole.sh`, which is the timer-not-a-loop pattern |

**This has been decided once already, and the decision has a condition on it.**
Issue #5, *"E3: one proven host per cloud, or drop it"*, closed on 2026-09-07
against exactly this: Fargate and Cloud Run became **recipes, not templates**,
because neither could be deployed from here and *"templates that look
authoritative and have never started a station"* is the thing E1 existed to
clean up. That reasoning is still right, and #49 raised the bar rather than
lowering it - every Azure template in the repository has now been deployed
live and torn down.

So the condition is an AWS account to prove against, not an argument about
whether AWS matters. **With one, this is the first host work. Without one, #5
stands and these stay recipes.** Writing three unproven templates would undo a
decision this repository has already paid for.

SSM stays what the master design already says it is: **an excellent installer
and a useless transport**, because AWS routes real output to a bucket and
truncated output is forbidden here.

## 2. Network devices

**Recommended second, and it is the answer to "any sort of device".** Almost
no new code: it is a proving exercise and a page.

Modern switch and router OSes are Linux with a shell, and they meet the five
points of the host contract:

| | |
|---|---|
| **Arista EOS** | `bash` from the enable prompt is documented and supported, in every mode except EXEC. Native `egrep` and `awk`. AAA can restrict it, which is the estate's decision and not ours |
| **Cisco IOS-XE Guest Shell** | an LXC container bundled with the image, enabled with `guestshell enable`, entered with `guestshell run bash`. CentOS-based, so bash 4. Survives a reload once enabled, and `bootflash:` is the shared directory. Not on Catalyst 9200L |
| **NVIDIA Cumulus Linux** | Debian. It is a server that forwards packets |
| **F5 BIG-IP** | bash beside tmsh |

**Two constraints shape which transport a network device can use.** Git is
frequently absent and installing it is out of the question, which points
straight at the artifact repository, the registry or the relay - the three
that need only curl. And the interesting reach is often in a management VRF,
so the transport's egress has to be selectable per source interface.

Guest Shell has one caveat worth carrying into the docs verbatim: Cisco warns
that scripts producing high-volume output can exhaust container memory, and
tells you to redirect to a file. The station already redirects to a file. That
is the capture contract doing its job on a device nobody wrote it for.

Sources: [Arista EOS CLI](https://www.arista.com/en/um-eos/eos-command-line-interface-cli),
[IOS-XE Guest Shell](https://www.cisco.com/c/en/us/td/docs/ios-xml/ios/prog/configuration/1718/b-1718-programmability-cg/guest_shell.html).

## 3. GitLab CI, and a Kubernetes CronJob

Two small ones that are close to free.

`station/bash/pipelines/` ships GitHub Actions and Azure Pipelines. **GitLab CI
is missing and GitLab is the git host of choice in a large part of the
regulated market**, so this is one YAML file against a pattern that already
exists, loop guard included.

`station/bash/kubernetes/heliograph.yaml` is a Deployment. A CronJob beside it
is the same image with `--once`, and it is what an estate that will not run a
long-lived pod will accept.

## 4. Ansible AWX, as a plant route, not a host

`heliograph plant --via ssh|azure-run-command|aws-ssm|manual|bundle` is the
right place for this, and AWX belongs in that list.

The shape matches the product exactly: an ops team has AWX precisely so that
nobody needs SSH, a job template is a reviewed and approved unit of work, and
launching one is a permission somebody already has. `plant` would print a job
template rather than a shell command.

**Note the honest limit**, because it is the same one the tool exists for: AWX
does not remove SSH, it moves it to the control plane. The station is still
planted by a play that reaches the host. What it removes is the *operator's*
need for a login, which is the thing that was blocking us.

## 5. Termux on Android

**Not recommended on value, recommended on evidence.** It costs a day and it
proves the sentence.

Termux has bash, GNU coreutils, `termux-services` on runit for a restart
policy, and Termux:Boot for start-on-reboot with `termux-wake-lock`. That is
four of the five host contract points already there, and the fifth is a
directory. A phone in a locked cabinet on a factory floor is a real station,
and it is the cheapest possible demonstration that the host contract means
what it says.

Source: [Termux:Boot](https://wiki.termux.com/wiki/Termux:Boot).

## Stations declined or parked

**BusyBox devices are declined, for now.** OpenWrt, most appliances and the
ESXi shell have ash and no bash, and bash-only forms like `${x/abc/xyz}` are
simply absent. Supporting them means a third station implementation in POSIX
`sh`, after bash and the PowerShell station that is not finished. The
PowerShell work has already shown what a second implementation of one file
costs: `.station-env` was written twice and the two disagreed six ways about
the same file. Do not start a third until the second is done.

**The mainframe and the midrange are parked as research.** z/OS UNIX System
Services has a maintained bash from Rocket and IBM i PASE installs one from
yum, and both are exactly the change-controlled, no-login estates this product
is for. **EBCDIC is why it is research and not a plan**: the whole capture
contract is a byte-for-byte claim about a text file, and a platform where git
itself tags and converts between IBM-1047 and ISO8859-1 will break the
redaction rules, the timestamp column, or both, in ways only a real run finds.
IBM i is the softer landing of the two: ASCII throughout PASE, with LF line
endings as the documented hazard.

Sources: [Rocket open source for z/OS](https://www.rocketsoftware.com/en-us/products/open-source/appdev-z),
[setting bash on IBM i](https://ibmi-oss-docs.readthedocs.io/en/latest/troubleshooting/SETTING_BASH.html).

---

## What to do first

Filed as issues #56 to #64. In this order, and the first two are the ones that
matter:

1. **The artifact repository transport** (#56). Largest population, lowest cost,
   `blob.sh` is the template. Prove the atomicity question with a round trip
   before writing the station side
2. **The AWS host family, if an account can be found for it** (#60). Fargate, then
   EC2, then Lambda, each deployed and torn down the way the Azure templates
   were. Without an account, issue #5 stands and they stay recipes
3. **Prove GCS against the existing object store** (#57), which is a CI job and may
   be a transport for nothing
4. **The network device page** (#61), once one of the curl-only transports is
   finished, because that is what a switch can actually use
5. **GitLab CI and the Kubernetes CronJob** (#58, #59), whenever somebody wants a small one
6. **The near side without the CLI** (#62), which is a page describing something that
   already works, and **Termux** (#63), which is a check on a target already built.
   **Claude Code on the web** (#64) is the same shape: prove the one transport
   its session proxy will allow, then say honestly what it refuses

Everything above the line in each section is a candidate, not a commitment.
The gating question for all of them is the one this repository keeps paying
for: **a transport that works on one side of the gap is not a transport**, and
neither is a host nobody has run a station on.
