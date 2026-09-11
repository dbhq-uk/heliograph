# What else could carry, run or drive heliograph

**Date:** 2026-09-10, with a postscript added 2026-09-11
**Source:** vendor documentation and protocol specifications, read live; plus
the repository itself, which answered more of the questions than the web did.
**Decisions:** none are taken here. The verdicts live in
[`../specs/2026-09-10-new-transports-and-stations-design.md`](../specs/2026-09-10-new-transports-and-stations-design.md)
and are published as [`/roadmap`](../../site/content/roadmap.md). This file is
the evidence they rest on, so that a verdict can be re-argued against what was
actually measured rather than against somebody's memory of it.
**Read section 8 first if the date above is not recent.** Three findings were
already overtaken within a day.

## 1. Summary

Four things came out of this that were not known before.

**The largest untapped population is the artifact repository.** An estate that
refuses a git host and refuses a storage account still runs Artifactory or
Nexus, because that is how software gets into the estate at all. Both are a
plain HTTP file store to curl. This was not on any list.

**Bandwidth is a gate, not a performance characteristic.** Every exotic
proposal - sound, radio, light - fails the same arithmetic, and the arithmetic
is in section 4. It is worth having as a standing test because these proposals
arrive regularly and each one is individually plausible.

**Network devices already meet the host contract.** Arista EOS, Cisco IOS-XE
Guest Shell, Cumulus and F5 BIG-IP all have bash. This costs no new code, and
it is the honest answer to "what sort of device can run a station".

**The repository answered the sharper questions.** Three of the most useful
findings came from reading the code, not the web: an AWS decision had already
been taken and closed (#5), the git transport already supports both schemes so
the real gap is the port underneath them, and a control node with no CLI at all
is already possible because the request document is hand-writable. Those are in
section 7.

## 2. What a candidate has to clear

Both contracts were already published and neither was written for this
exercise. They are restated because they did most of the filtering.

A **transport** implements ten station-side functions, seven on the capture
path, and its `tp_check` proves a write rather than a read. A **host** needs a
process that can run bash, outbound reach to one transport, a restart policy, a
non-root account, and somewhere to write a file. Behind both sits the
constraint that decides more cases than either: **nothing is ever installed on
the far side**.

## 3. Transports, measured

### Artifact repositories

Artifactory deploys and retrieves with an ordinary HTTP verb:
`curl -u user:token -X PUT -T file 'https://<host>/artifactory/<repo>/<path>'`,
and a GET on the same URL reads it back. Matrix parameters can be appended to
record provenance. An access token is the documented credential for automation;
the `X-JFrog-Art-Api` header in older guides is the legacy API key.

Nexus raw repositories are the same shape at `/repository/<repo>/<path>`, with
`curl --upload-file` issuing the PUT and the GET path identical to the PUT
path. Two recorded traps: a trailing-slash target URL puts the asset in the
repository root rather than the intended directory, and the Components REST API
form silently uploads empty files if the `@` prefix is omitted from the file
field.

**The open question, and it is the important one:** what a GET sees during an
in-flight PUT. JFrog documents checksum-based storage and an atomic flag for
exploded archives, and neither statement covers concurrent readers; Nexus does
not address it either. This matters because a log that ends mid-line with no
footer is the shape an operator reads as *still running*. **Settle it with a
round trip.** The file share transport learned the same lesson the expensive
way.

Worth knowing if anyone reaches for the vendor CLI instead of curl: `jf rt curl`
**always exits 0** regardless of HTTP status, so a 401 or a 5xx looks like
success to a script.

### OCI registries

A registry has been a general artifact store since ORAS, and an artifact no
longer has to be wrapped in an image to be pushed. Underneath, it is the OCI
distribution specification and three HTTP calls:

```
POST /v2/<name>/blobs/uploads/            open a session
PUT  <location>?digest=sha256:<hex>       the content, as a blob
PUT  /v2/<name>/manifests/<tag>           a manifest naming it
```

Reading back is trivially curl-able. The two hard parts, both known in advance,
are the `WWW-Authenticate` challenge and token exchange - which every registry
does slightly differently, and which ORAS exists to hide - and computing
digests, which the station's preflight already provides a sha256 for.

Support is broad: ECR, ACR, Google Artifact Registry, GitHub, and self-hosted
distribution or zot. Google's own documentation points at the distribution spec
directly when other clients are unsuitable.

### Microsoft Graph

The route is site to drive to item, all curl-able with a bearer token.
Download is `GET /drives/{drive}/items/{item}/content`, and Graph answers with
a 302 to a pre-authenticated URL, so `-L` is required.

Upload splits at **4MB**, which is the finding that matters. Under it,
`PUT /drives/{drive}/items/{parent}:/{name}:/content`. Over it, a
`createUploadSession` returning an upload URL that takes `Content-Range`
headers and, being pre-authenticated, does **not** take the bearer token.
Chunks may not exceed 60MB. A captured log crosses 4MB easily, so the session
path is the normal case rather than an edge.

Auth: device code suits an operator's first run and needs "allow public client
flows" on the app registration. It is a poor fit for anything that has to
survive a reboot unattended, which points at client credentials with a
certificate and a refresh path no existing transport here has.

### Google Cloud Storage

GCS accepts AWS Signature V4 on its S3-compatible XML API with HMAC keys, and
`internal/transport/objstore.go` already signs SigV4. This is the cheapest
candidate found: it may already work with `--dir https://storage.googleapis.com
--region auto` and no code. **It was not tested**, and it should not enter any
table until a round trip says so.

### Ticketing systems

ServiceNow exposes attachments over plain REST - `POST /api/now/attachment/file`
with `table_name`, `table_sys_id` and `file_name` as query parameters, or a
multipart form to `/api/now/attachment/upload`, and
`GET /api/now/attachment/{sys_id}/file` to read one back. The `download_link`
field in attachment metadata is unreliable; the `/file` endpoint is not.

Two documented traps: ServiceNow's own API Explorer emits **incorrect curl
snippets** for attachment upload (tracked as PRB646918 / KB0551442), and
buffering a file to disk before upload has produced corrupt attachments where
streaming it directly did not.

## 4. The bandwidth arithmetic

The exotic channels all fail the same test, so the numbers are recorded
together.

| channel | throughput | a 50KB log takes |
|---|---|---|
| ggwave, data over sound | 8-16 bytes/s | 53 minutes to 1 hour 47 |
| Meshtastic, Very Long Slow | ~11 bytes/s | 1 hour 17 |
| Meshtastic, Long Fast (default) | ~84 bytes/s | 10 minutes |
| Meshtastic, Short Turbo | ~2.7 KB/s | 19 seconds |
| animated QR, v40 at 10 fps | ~29 KB/s before overhead | under 2 seconds |

**ggwave** is FSK with Reed-Solomon, 8-16 bytes/s by its own documentation, at
1 to 5 metres indoors. A third-party claim of 500 bytes/s conflicts with the
project's own figure and an independent academic comparison putting it at 128
bps, so the project's number is the one used here. It does not handle audio
playback or capture, so speaker and microphone rolloff near 18-20kHz is the
integrator's problem. Related work reaches further by exploiting microphone
nonlinearity - BackDoor at 4 kbps, BatComm at 47 kbps at 10cm - but BatComm is
not open source.

**Meshtastic** figures are theoretical maxima from the Semtech calculator and
exclude headers, mesh hops and retransmissions. EU_433 and EU_868 carry a 10%
hourly duty cycle limit on a rolling basis. A position packet is around 40
bytes and already costs seconds of airtime on the default preset.

**Animated QR** is the only one that clears. A version 40 QR holds 2953 binary
symbols. txqr moved from a naive repeating sequence to LT fountain codes
precisely because a single missed frame otherwise costs a whole cycle, and
qrxfer - which is explicitly framed as air-gap transfer - records that two
screens facing each other was far less error-prone than a handheld phone. One
recent browser implementation claims nearly 190 KB/s using a WASM ZXing build
with workers dropping excess frames and the fountain layer absorbing the loss.

**The asymmetry is what makes QR interesting and limited.** A request is a few
hundred bytes and fits in one static code. A log needs the far side to render
animated frames, which is a package and a display a station does not have.

## 5. Transfer tools that are not transports

rclone, Syncthing, croc and magic-wormhole all came up and all fail the same
clause. Each is a binary or a daemon on the far side.

croc and magic-wormhole are additionally the relay again but worse: both are
rendezvous-based, so **both ends must be online at the same moment**, and
heliograph is store-and-forward on purpose. rsync, rclone and Syncthing are
synchronisation engines rather than send-a-thing tools, and rsync needs SSH and
a peer process, which is the thing this product exists because you do not have.

The useful residue is one documentation sentence: an operator who already runs
rclone has a file share or an object store, and the existing transports work
through it without heliograph needing to know.

## 6. Hosts

**Network devices.** Arista documents and supports bash: it is reached with
`bash` from the enable prompt, available in every command mode except EXEC,
with `egrep` and `awk` present, and AAA can restrict it. Arista does not
recommend updating the underlying Linux, though YUM is supported for
extensions.

Cisco IOS-XE Guest Shell is an LXC container bundled with the image, enabled
with `guestshell enable` and entered with `guestshell run bash`, historically
CentOS 7 based. It shares the kernel and cannot modify the host filesystem.
Persistence has a specific shape: disabling it survives a reload, a directory
on flash is synchronised across stack members and is where data must go, and on
HA switchover the new active builds its own installation and restores to the
synchronised state. `/bootflash` inside the container is IOS-XE's `bootflash:`.
Not supported on Catalyst 9200L. **Cisco's own caveat is worth quoting**: a
script running high-volume commands at scale can exhaust container memory, and
output should be redirected to a file - which is what the capture contract
already does.

**z/OS and IBM i.** Bash exists on both, and both are exactly the
change-controlled estate this product targets. Rocket Software maintains the
z/OS port, shipped in Open AppDev for Z alongside git, curl and Python. The
hazard is EBCDIC: the z/OS git port tags and converts text between IBM-1047 and
ISO8859-1 automatically, and the capture contract is a byte-for-byte claim
about a text file. IBM i PASE is the softer landing - `yum install bash`,
binaries under `/QOpenSys/pkgs/bin`, `chsh` to make it the default - with LF
line endings as the documented trap, since a Windows-saved script fails with
"bad interpreter".

**BusyBox devices.** OpenWrt and similar have ash, not bash, and bash-only
forms such as `${x/abc/xyz}` are simply absent. Supporting them means a third
station implementation in POSIX `sh`. `checkbashisms` and ShellCheck are the
tools if it is ever attempted.

**Termux.** Four of the five host contract points are already present:
`termux-services` provides runit supervision with `sv-enable` and `sv up`, and
Termux:Boot runs `~/.termux/boot/` scripts on reboot, after `termux-wake-lock`
and roughly 60 seconds after unlock, with auto-start permitted in Android's
per-app settings. One reported friction: plain bash loops under `sv up` have
needed `daemonize` as a fallback.

**Ansible AWX.** A job template is a reviewed, approved unit of work that an
operator can launch without touching a key, and credentials are injected at
runtime into an ephemeral execution environment. **The honest limit:** AWX does
not remove SSH, it moves it to the control plane. What it removes is the
operator's need for a login. Exposing a free-text command survey would hand
back exactly the remote execution the control was meant to prevent.

## 7. What the repository said, which the web could not

The most useful findings of the exercise came from reading the code.

**An AWS decision was already taken.** Issue #5, *"E3: one proven host per
cloud, or drop it"*, closed on 2026-09-07 making Fargate and Cloud Run recipes
rather than templates, because neither could be deployed from here and
*"templates that look authoritative and have never started a station"* is what
E1 existed to clean up. #49 then raised the bar: every Azure template has been
deployed live and torn down. So AWS is conditional on an account, not on an
argument.

**Both git schemes are already supported.** `tp_preflight` branches on the
remote (`git@*|ssh://*` against `https://*`), reports the ssh-agent state with
four distinct outcomes and remedies, and reports the https token by mechanism
and length. The gap is a layer below: a station behind a firewall that drops 22
gets `ls-remote failed: ... Connection timed out. Check the remote URL and the
credential reported above`, and both are fine. Nothing in the repository
mentions `ssh.github.com:443` or `altssh.gitlab.com:443`, which are the remedy
and need no code. Filed as #66.

**A control node needs no CLI.** A request is a `key: value` text document, and
`wire.Request.Marshal` writes every key even when empty, commented *"sed does
not need them, but the operator reading the file does"*. So a laptop that
permits git and refuses new binaries is already a control node. Filed as #62.

**Only git shells out.** `internal/transport/git.go` calls `exec.Command("git")`;
relay, share, object store and bundle are pure Go. A machine with no git can
still drive an estate.

**The near side is already six targets.** `release.yml` builds linux, darwin
and windows on both amd64 and arm64, static, `CGO_ENABLED=0`. Estates live in
`$XDG_CONFIG_HOME/heliograph/estates`, which matters for any ephemeral control
node.

**Claude Code on the web is a better fit than a remote MCP connector.** A cloud
sandbox is a real shell, so the static binary simply runs. What stops it is the
session proxy: each session gets a fresh sandbox behind an Anthropic-managed
allowlist, refusals return 403 with `x-deny-reason: blocked-by-allowlist`, and
there are open reports that "All domains" is not reflected in the session JWT,
which hits private and self-hosted infrastructure specifically. Custom
connectors do span web, Desktop and mobile, but reach the server **from
Anthropic's cloud even on Desktop**, so they need a public endpoint - which
would hold the transport credential and, on the relay, the signing identity.
The MCP authorization specification forbids token passthrough for the adjacent
reason, and names the confused deputy problem explicitly. Filed as #64.

## 8. What changed after this was taken

Left as a postscript rather than edited into the text above, because a
measurement that is quietly updated is one nobody can check.

- **2026-09-11: Track B finished.** PR #73 landed `station.ps1`, `run.ps1`,
  `start.ps1`, bootstrap for both flavours and the receive half of git and
  share, with conformance passing all ten properties on Windows PowerShell 5.1
  and 7 with **zero skips**. It also found five defects, three in code that had
  already shipped, including `./station.sh --allow-root` never having worked
  because it set a shell variable nothing exported. The tracking issue filed
  here on 2026-09-10 was closed the next day as already false
- **The PowerShell relay transport is what remains**, deferred deliberately
  because it needs `heliograph-seal`, and a station that must ship a Go binary
  is a different bootstrap question on exactly the estates that will not install
  Git for Windows. Filed as #77
- **2026-09-11: HTTPS is now enforced on the site.** `http://heliograph.dbhq.uk/`
  returned 200 with no redirect when #70 was written and returns 301 now,
  though no `Strict-Transport-Security` header came back on the response
  checked

## 9. Sources

Read live on 2026-09-10 unless stated.

| | |
|---|---|
| Artifactory | [cURL integration](https://docs.jfrog.com/integrations/docs/curl-integration), [deploy artifacts](https://docs.jfrog.com/artifactory/docs/deploy-artifacts) |
| Nexus | [raw repositories](https://help.sonatype.com/en/raw-repositories.html) |
| OCI and ORAS | [oras.land](https://oras.land/docs/), [pushing and pulling](https://oras.land/docs/how_to_guides/pushing_and_pulling/), [ACR artifacts](https://learn.microsoft.com/en-us/azure/container-registry/container-registry-manage-artifact) |
| Microsoft Graph | [upload small files](https://learn.microsoft.com/en-us/graph/api/driveitem-put-content?view=graph-rest-1.0), [download content](https://learn.microsoft.com/en-us/graph/api/driveitem-get-content?view=graph-rest-1.0) |
| ggwave | [ggerganov/ggwave](https://github.com/ggerganov/ggwave) |
| animated QR | [divan/txqr](https://github.com/divan/txqr), [leonjza/qrxfer](https://github.com/leonjza/qrxfer) |
| Meshtastic | [LoRa configuration](https://meshtastic.org/docs/configuration/radio/lora/), [radio settings](https://meshtastic.org/docs/overview/radio-settings/) |
| Arista EOS | [command-line interface](https://www.arista.com/en/um-eos/eos-command-line-interface-cli) |
| Cisco Guest Shell | [programmability configuration guide 17.18](https://www.cisco.com/c/en/us/td/docs/ios-xml/ios/prog/configuration/1718/b-1718-programmability-cg/guest_shell.html) |
| z/OS and IBM i | [Rocket Open AppDev for Z](https://www.rocketsoftware.com/en-us/products/open-source/appdev-z), [setting bash on IBM i](https://ibmi-oss-docs.readthedocs.io/en/latest/troubleshooting/SETTING_BASH.html) |
| Termux | [Termux:Boot](https://wiki.termux.com/wiki/Termux:Boot), [termux-services](https://github.com/termux/termux-services) |
| MCP | [connect to remote servers](https://modelcontextprotocol.io/docs/develop/connect-remote-servers), [security best practices](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices), [custom connectors](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp) |
| Claude Code sandbox | [sandboxing docs](https://code.claude.com/docs/en/sandboxing), [proxy allowlist issue #52982](https://github.com/anthropics/claude-code/issues/52982) |
| ServiceNow | [attachment API community threads](https://www.servicenow.com/community/developer-forum/attach-a-file-using-rest-api/m-p/2187146) |
