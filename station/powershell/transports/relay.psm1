# =============================================================================
#  transports/relay.psm1 - the relay, with no binary at all
# =============================================================================
# The twin of transports/relay.sh, for an estate that will not give you a git
# host, a storage account, a share or an inbound route: both sides dial OUT over
# ordinary HTTPS and meet at a server neither of them trusts.
#
# THE RELAY IS NOT TRUSTED, IN EITHER DIRECTION. Everything is sealed and signed
# by the far side before it is sent and verified here before it is acted on. A
# relay that could merely read logs would be a privacy problem; a relay that
# could FORGE a request would have code execution inside every estate at once,
# through a channel the estate installed deliberately. So authenticity is
# checked first and confidentiality second.
#
# WHAT IS DIFFERENT FROM THE BASH TWIN, and it is the reason this file exists.
#
# `relay.sh` shells out to `heliograph-seal`, a native Go binary, because a
# shell cannot do AEAD - `openssl enc` refuses AEAD ciphers outright. This
# payload is for estates that will not let you install a native binary at all,
# so the same reasoning would have left them with no relay for ever.
#
# It does not. lib/seal.psm1 does the construction in managed code over
# vendored Chaos.NaCl, and it is held to `tests/fixtures/seal-vectors.json` -
# fixed bytes emitted by the Go side - so the two implementations are compared
# rather than assumed to agree. There is no RELAY_SEAL, no checksum to verify,
# and nothing to install.
#
# The networking stays here, in PowerShell, exactly as curl stays in the shell
# on the bash side: the seal seals, and the thing that talks to the internet is
# readable by the operator running it.
#
# NO PLAINTEXT FALLBACK AND NO OPPORTUNISTIC MODE. A station configured for the
# relay either speaks sealed or does not speak.
# =============================================================================

Set-StrictMode -Version 2.0

$script:Base = ''
$script:Estate = ''
$script:Station = ''
$script:Identity = $null
$script:Peer = $null
$script:StatePath = ''

# No `self`: a payload update arrives as a sealed message, which is the same
# --allow-payload capability as anywhere else and is off by default. No `live`
# either - a mid-run poll costs a request per interval against a metered edge,
# and the cancel it would deliver can wait for the next cycle. Same list as
# relay.sh, and the loop holds a transport to exactly what it declares.
function Get-TpCapabilities { return 'request status progress' }

function Initialize-Tp {
    foreach ($n in @(
            @('RELAY_URL', 'the base URL of the relay both sides dial out to'),
            @('RELAY_ESTATE', 'which estate this station belongs to'),
            @('RELAY_STATION', 'the station name, which is what a run is bound to'),
            @('RELAY_TOKEN', 'the station-scoped token; without it every request is refused and it reads like a fault at the far end'),
            @('RELAY_IDENTITY', "this station's key file"),
            @('RELAY_PEER', "the control side's public identity, which is what a request is verified against"))) {
        if (-not (Test-TpNeed -Name $n[0] -Why $n[1])) { return $false }
    }

    # INTERPOLATED INTO A URL, so they are checked before they become one. A `/`
    # in either reaches a different route, a `?` starts a query string and a `#`
    # truncates the path - all silently, and all ending as a 404 or as somebody
    # else's queue. relay.sh and the control side apply the same rule.
    foreach ($v in 'RELAY_ESTATE', 'RELAY_STATION') {
        $val = [System.Environment]::GetEnvironmentVariable($v)
        if ($val -cnotmatch '^[A-Za-z0-9._-]+$' -or $val.StartsWith('-')) {
            Write-CapTpError "$v is '$val', which is not a usable routing key. It becomes part of a URL path, so it may hold only letters, digits, dot, hyphen and underscore, and may not begin with a hyphen"
            return $false
        }
    }

    $script:Base = $env:RELAY_URL.TrimEnd('/')
    $script:Estate = $env:RELAY_ESTATE
    $script:Station = $env:RELAY_STATION

    # THE SEAL BEFORE THE NETWORK. Add-Type compiles sixty files here, and a
    # payload missing them must say so as a transport problem rather than as a
    # method-not-found three calls later.
    try {
        Import-Module (Join-Path (Split-Path -Parent $PSScriptRoot) 'lib/seal.psm1') -Force -Global -ErrorAction Stop
        Initialize-Seal
    } catch {
        Write-CapTpError "the seal will not load, so this station could neither verify a request nor seal a log: $($_.Exception.Message)"
        return $false
    }

    # BOTH KEY FILES, AND PARSED RATHER THAN MERELY READ. Readable is not
    # usable: a truncated peer key, or the wrong file entirely, is readable and
    # then fails every verification afterwards - on a machine nobody can log
    # into. relay.sh learned this by shipping a station that started perfectly
    # with an unreadable RELAY_PEER and failed every single request.
    #
    # NAMES THE VARIABLE, not only the path. They are set by different people at
    # different times: the identity is made on this machine, the peer arrives
    # from the control side.
    try {
        $script:Identity = Import-SealIdentityFile $env:RELAY_IDENTITY
    } catch {
        Write-CapTpError "cannot use RELAY_IDENTITY at $($env:RELAY_IDENTITY) - that is this station's own key: $($_.Exception.Message)"
        return $false
    }
    try {
        $script:Peer = Import-SealPublicFile $env:RELAY_PEER
    } catch {
        Write-CapTpError "cannot use RELAY_PEER at $($env:RELAY_PEER), and that file is what a request is verified against. Without it this station can neither accept a request nor seal a log: $($_.Exception.Message)"
        return $false
    }

    # $PSScriptRoot is transports/, so its parent is the payload root - the same
    # place relay.sh puts it, which matters because an operator who has been
    # told where to look for one will look there for the other.
    $script:StatePath = $env:RELAY_STATE
    if (-not $script:StatePath) {
        $script:StatePath = Join-Path (Split-Path -Parent $PSScriptRoot) '.station-relay-state'
    }

    # TLS 1.2 ADDED, NOT ASSIGNED. Windows PowerShell 5.1 on .NET Framework
    # defaults to SSL3 and TLS 1.0, which every relay worth dialling refuses -
    # and the failure is "the underlying connection was closed", which names
    # nothing. Assigning instead of OR-ing would take 1.3 away on PowerShell 7,
    # so the existing value is kept and 1.2 is added to it.
    try {
        [Net.ServicePointManager]::SecurityProtocol =
            [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    } catch {
        # Not fatal, and not silent. On a runtime where this property is
        # managed by the OS there is nothing to set and nothing wrong.
        Write-CapTpError "could not raise the TLS floor to 1.2: $($_.Exception.Message)"
    }
    return $true
}

function Get-TpScope { return $script:Station }
function Get-TpRevision { return "relay $($script:Base), estate $($script:Estate)" }
function Get-TpDescribe {
    $fp = try { Get-SealFingerprint $script:Peer } catch { '<unreadable>' }
    return "relay $($script:Base), estate $($script:Estate), station $($script:Station), peer $fp"
}

# =============================================================================
#  The sequence numbers, across two processes
# =============================================================================
# THE DEFECT THIS EXISTS TO PREVENT, which made the bash relay unusable and
# looked like nothing at all.
#
# The loop and the runner are SEPARATE PROCESSES. The loop reads the state once
# and holds it; the runner is a child that loads the transport of its own in
# order to deliver the finished log. So the runner published the log as sequence
# N and the loop then published `idle` as N as well, from the value it had
# loaded before the step started. The receiver drops anything at or below what
# it has already accepted - by design, because a replayed request is a
# destructive step re-running with its gates already satisfied - so the `idle`
# went into the relay and out of existence. From the control node: the log
# arrives and the station reports `running` for ever, with nothing erroring
# anywhere.
#
# THE LOCK IS AN EXCLUSIVELY-OPENED FILE, not relay.sh's mkdir-and-a-pid dance.
# relay.sh has to ask `kill -0` whether the recorded pid is alive, because a
# directory left behind by a dead holder looks exactly like one held by a live
# one. A file opened with FileShare.None needs none of that: the operating
# system releases the handle when the process dies, however it dies, so the next
# open succeeds. The bash version is careful because it has to be; this one is
# simple because it can be.
function Enter-RelayLock {
    $lock = "$($script:StatePath).lock"
    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    while ($true) {
        try {
            return [System.IO.File]::Open($lock, [System.IO.FileMode]::OpenOrCreate,
                [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
        } catch [System.IO.IOException] {
            # A SHARING VIOLATION AND A MISSING DIRECTORY ARE BOTH IOException,
            # and only one of them is worth waiting for. Waiting on the other is
            # how a station stops publishing for ever with nothing in the log.
            if (-not (Test-Path -LiteralPath (Split-Path -Parent $lock) -PathType Container)) {
                Write-CapTpError "cannot create the sequence lock in $(Split-Path -Parent $lock) - the directory is not there"
                return $null
            }
            if ([DateTime]::UtcNow -gt $deadline) {
                Write-CapTpError "gave up waiting for the sequence lock at $lock after 30 seconds"
                return $null
            }
            Start-Sleep -Milliseconds 50
        } catch {
            Write-CapTpError "cannot take the sequence lock at $lock : $($_.Exception.Message)"
            return $null
        }
    }
}

function Get-RelayField {
    param([string] $Name)
    try {
        foreach ($line in [System.IO.File]::ReadAllLines($script:StatePath)) {
            if ($line.StartsWith("$Name`:")) {
                $v = $line.Substring($Name.Length + 1)
                if ($v -cmatch '^[0-9]+$') { return [UInt64] $v }
                return [UInt64] 0
            }
        }
    } catch { }
    return [UInt64] 0
}

# Write both fields, taking the HIGHER of what is on disk and what is offered.
#
# NEVER A PLAIN OVERWRITE. The runner does not fetch requests, so its idea of
# `seen` is whatever the loop had when the step started - and writing that back
# would REGRESS the replay counter, which is the one number whose whole job is
# never to go backwards.
#
# Write-then-rename, because a truncated state file reads as sequence zero, and
# sequence zero accepts every replay the relay has ever seen.
function Save-RelayState {
    param([UInt64] $Seen = 0, [UInt64] $Out = 0)
    $fseen = Get-RelayField 'seen'
    $fout = Get-RelayField 'out'
    if ($Seen -gt $fseen) { $fseen = $Seen }
    if ($Out -gt $fout) { $fout = $Out }
    # Named for this process: two writers on one path make the loser's move
    # fail, which shows up only as a state file that was not updated.
    $tmp = "$($script:StatePath).$PID.tmp"
    try {
        [System.IO.File]::WriteAllText($tmp, "seen:$fseen`nout:$fout`n")
        # RENAMED, NOT COPIED. A copy is not atomic: a crash or a full disk
        # part way through leaves a TRUNCATED state file, and a truncated one
        # reads as sequence zero - which accepts every replay the relay has
        # ever held. relay.sh writes then `mv -f` for exactly this reason.
        #
        # Two calls because .NET Framework has no File.Move overwrite overload
        # - that arrived in .NET Core 3.0, and this has to run on 5.1. Replace
        # is the atomic one and needs the destination to exist; Move is for the
        # first write, when it does not.
        #
        # [NullString]::Value AND NOT $null FOR THE BACKUP PATH. PowerShell
        # marshals $null to an EMPTY STRING when the parameter is typed
        # `string`, and Replace refuses that with "The value cannot be an empty
        # string. (Parameter 'path')" - so every write after the first failed,
        # the sequence number never advanced past 1, and every delivery after
        # the first was refused. Conformance did not catch it: p9 delivers once
        # per bootstrap, so the second write never happened.
        if ([System.IO.File]::Exists($script:StatePath)) {
            [System.IO.File]::Replace($tmp, $script:StatePath, [NullString]::Value)
        } else {
            [System.IO.File]::Move($tmp, $script:StatePath)
        }
        return @{ Ok = $true; Seen = $fseen; Out = $fout }
    } catch {
        try { [System.IO.File]::Delete($tmp) } catch { }
        # THE RESULT IS RETURNED AND THE CALLERS CHECK IT. A full disk or a
        # read-only mount made the bash version fail silently, so a request was
        # verified and RUN with its sequence number never persisted - and after
        # the next restart the relay could replay that request and have it
        # accepted again. The replay defence is only a defence if it is written
        # down.
        Write-CapTpError "could not persist the relay sequence state at $($script:StatePath): $($_.Exception.Message)"
        return @{ Ok = $false; Seen = $fseen; Out = $fout }
    }
}

function Get-RelayNextOut {
    $lock = Enter-RelayLock
    if (-not $lock) { return $null }
    try {
        $next = (Get-RelayField 'out') + 1
        $r = Save-RelayState -Out $next
        # A number that could not be recorded has not been reserved: the next
        # process hands out the same one and the receiver drops everything after
        # the first. Fail rather than send under a number nothing is holding.
        if (-not $r.Ok) { return $null }
        return $next
    } finally { $lock.Dispose() }
}

function Set-RelaySeen {
    param([UInt64] $Seq)
    $lock = Enter-RelayLock
    if (-not $lock) { return $false }
    try { return (Save-RelayState -Seen $Seq).Ok } finally { $lock.Dispose() }
}

function Get-RelayUrl { param([string] $Leaf) return "$($script:Base)/v1/$($script:Estate)/$($script:Station)/$Leaf" }

# =============================================================================
#  The wire
# =============================================================================
# Invoke-WebRequest and not Invoke-RestMethod: the body here is an opaque blob,
# and Invoke-RestMethod would try to parse it as JSON and hand back an object.
#
# -UseBasicParsing because Windows PowerShell 5.1 otherwise instantiates the
# Internet Explorer engine to parse the response - which on Server Core is not
# there, and the failure names neither IE nor the flag.
function Invoke-Relay {
    param([string] $Method, [string] $Uri, [byte[]] $Body, [int] $TimeoutSec = 60, [switch] $NoAuth)
    $headers = @{}
    if (-not $NoAuth) { $headers['Authorization'] = "Bearer $($env:RELAY_TOKEN)" }
    # NOT NAMED $args. That is the automatic variable holding this function's
    # own arguments; assigning to it works and then splatting it does something
    # other than what the line appears to say.
    $call = @{
        Method = $Method; Uri = $Uri; Headers = $headers
        TimeoutSec = $TimeoutSec; UseBasicParsing = $true; ErrorAction = 'Stop'
    }
    if ($null -ne $Body) {
        $call['Body'] = $Body
        $call['ContentType'] = 'application/json'
    }
    try {
        $r = Invoke-WebRequest @call
        return @{ Code = [int] $r.StatusCode; Content = $r.Content }
    } catch [System.Net.WebException] {
        $resp = $_.Exception.Response
        if ($resp) { return @{ Code = [int] $resp.StatusCode; Content = '' } }
        return @{ Code = 0; Content = ''; Error = $_.Exception.Message }
    } catch {
        # PowerShell 7 raises HttpResponseException rather than WebException,
        # and the status is on a different property. Both editions run this
        # file, so both shapes are handled rather than one being assumed.
        $r = $_.Exception.PSObject.Properties['Response']
        if ($r -and $r.Value) { return @{ Code = [int] $r.Value.StatusCode; Content = '' } }
        return @{ Code = 0; Content = ''; Error = $_.Exception.Message }
    }
}

function Test-Tp {
    # THE KEYS FIRST, before any network. Already parsed in Initialize-Tp, so
    # this asserts they are still there rather than re-deriving them.
    if (-not $script:Peer -or -not $script:Identity) {
        Write-CapTpError 'the relay identities are not loaded, so nothing could be verified'
        return $false
    }

    $h = Invoke-Relay -Method GET -Uri "$($script:Base)/health" -TimeoutSec 20 -NoAuth
    if ($h.Code -ne 200) {
        Write-CapTpError "relay health: HTTP $($h.Code)$(if ($h.ContainsKey('Error')) { " - $($h.Error)" })"
        return $false
    }

    # Health needs no token, so it proves reachability and nothing else. Prove
    # the credential too, or a wrong one is discovered by a failed log push an
    # hour from now with nobody left to tell.
    #
    # AGAINST A STATION NAME NOTHING USES. relay.sh probed this station's own
    # queue, and the relay DELETES ON COLLECTION - so every preflight silently
    # ate whatever request was waiting, and the station came up reporting itself
    # idle for ever with the request gone. Tokens are scoped to the ESTATE, so
    # an unused routing key proves exactly the same thing.
    #
    # `~` cannot begin a station name - the whitelist above forbids it - so this
    # can never collide with a real one, and `~` is unreserved in a URL path.
    $p = Invoke-Relay -Method GET -TimeoutSec 20 `
        -Uri "$($script:Base)/v1/$($script:Estate)/~heliograph-preflight/c2s?wait=0"
    switch ($p.Code) {
        200 { return $true }
        401 { Write-CapTpError "the relay is reachable but refused this token for estate $($script:Estate)"; return $false }
        403 { Write-CapTpError "the relay is reachable but refused this token for estate $($script:Estate)"; return $false }
        default { Write-CapTpError "relay: HTTP $($p.Code)"; return $false }
    }
}

# =============================================================================
#  Receive
# =============================================================================
# `$null` MEANS FAILED and '' means "nothing queued", the same split every other
# PowerShell transport keeps. Collapsing them is how a station goes permanently
# deaf without anybody being told.
#
# A MESSAGE THAT DOES NOT VERIFY IS DROPPED, not reported as an error. The relay
# is entitled to hand us anything, and a station that stopped on rubbish is one
# a hostile relay could halt at will.
function Receive-TpRequest {
    $r = Invoke-Relay -Method GET -Uri "$(Get-RelayUrl 'c2s')?wait=0" -TimeoutSec 40
    if ($r.Code -eq 0) {
        Write-CapTpError "the relay could not be reached: $($r.Error)"
        return $null
    }
    if ($r.Code -ne 200) {
        Write-CapTpError "relay fetch: HTTP $($r.Code)"
        return $null
    }
    if (-not $r.Content) { return '' }

    # A JSON ARRAY of {seq, body}, which is the relay's wire shape - the same
    # one `heliograph-seal open` and Relay.collect parse. The body is standard
    # base64 with padding, because that is what Go's json.Marshal does to a
    # []byte, and NOT the base64url the identities use. Two encodings in one
    # protocol is a trap, so it is named here rather than inferred.
    try { $msgs = @([string] $r.Content | ConvertFrom-Json) } catch {
        # Not an error. Anything unparseable is "nothing usable in the relay's
        # answer", and a station that stopped on rubbish is one a hostile relay
        # could halt at will.
        return ''
    }
    if ($msgs.Count -eq 0) { return '' }

    $seen = Get-RelayField 'seen'
    $bestSeq = [UInt64] 0
    $best = $null

    # SORTED, AND THE NEWEST THAT OPENS WINS. The relay may hand back several -
    # a status the control side queued behind a request, or a retry - and the
    # order it hands them back in is the relay's choice, which is exactly the
    # thing not to trust.
    foreach ($msg in ($msgs | Sort-Object { [UInt64] $_.seq })) {
        if (-not $msg.PSObject.Properties['seq'] -or -not $msg.PSObject.Properties['body']) { continue }
        $seq = [UInt64] $msg.seq
        # THE REPLAY FLOOR. A replayed request is a destructive step re-running
        # with its gates already satisfied, so anything at or below what has
        # been accepted is dropped without being opened.
        if ($seq -le $seen) { continue }

        # THE SEQUENCE THE RELAY CLAIMS IS NOT TRUSTED EITHER - it is used to
        # build the metadata, and the metadata is a SIGNED field bound into the
        # key. A relay that renumbered a message makes it fail to decrypt, which
        # is the point of binding the metadata into the key rather than only
        # into the signature.
        $m = New-SealMeta -Estate $script:Estate -Station $script:Station -Dir 'c2s' `
            -Seq $seq -Kind 'request' -Recipient (Get-SealFingerprint $script:Identity)
        try {
            $sealed = [Convert]::FromBase64String([string] $msg.body)
            $plain = Unprotect-Seal -To $script:Identity -ExpectFrom $script:Peer -Meta $m -Sealed $sealed
        } catch { continue }
        $bestSeq = $seq
        $best = $plain
    }

    if ($null -eq $best) { return '' }
    # RECORDED BEFORE IT IS RETURNED. A request that ran under a sequence number
    # nothing wrote down can be replayed after a restart and accepted again,
    # with its gates already satisfied.
    if (-not (Set-RelaySeen -Seq $bestSeq)) { return $null }
    return [System.Text.Encoding]::UTF8.GetString($best)
}

# Declared absent from Get-TpCapabilities, so the loop never calls these.
# Defined anyway, so calling one is a refusal rather than "the term is not
# recognised" - which reads as a broken payload rather than a transport saying
# no.
function Receive-TpRequestLive { return $null }
function Sync-TpSelf { return 1 }

# =============================================================================
#  Send
# =============================================================================
function Send-RelaySealed {
    param([string] $Kind, [byte[]] $Plaintext)
    # TAKEN UNDER THE LOCK, not incremented from a value this process loaded at
    # start. See the note above Enter-RelayLock.
    $seq = Get-RelayNextOut
    if ($null -eq $seq) { return $false }

    $m = New-SealMeta -Estate $script:Estate -Station $script:Station -Dir 's2c' `
        -Seq $seq -Kind $Kind -Recipient (Get-SealFingerprint $script:Peer)
    try {
        $sealed = Protect-Seal -From $script:Identity -To $script:Peer -Meta $m -Plaintext $Plaintext
    } catch {
        Write-CapTpError "could not seal the $Kind : $($_.Exception.Message)"
        return $false
    }

    # THE RELAY'S WIRE SHAPE, not the raw envelope. `{"seq":N,"body":"..."}`,
    # with the body in STANDARD base64 with padding - that is what Go's
    # json.Marshal produces for a []byte, and it is a different alphabet from
    # the base64url the identities use. Posting the raw bytes gets a 400 from
    # the relay and reads as a sealing failure.
    #
    # Hand-built rather than ConvertTo-Json, because Windows PowerShell 5.1
    # renders a UInt64 through its own serialiser and the seq must be a bare
    # JSON number. The two fields are a number and base64, so neither can
    # contain anything that needs escaping.
    $wire = [System.Text.Encoding]::UTF8.GetBytes(
        '{"seq":' + $seq + ',"body":"' + [Convert]::ToBase64String($sealed) + '"}')

    $r = Invoke-Relay -Method POST -Uri (Get-RelayUrl 's2c') -Body $wire -TimeoutSec 60
    if ($r.Code -ne 202) {
        Write-CapTpError "relay put ($Kind): HTTP $($r.Code)$(if ($r.ContainsKey('Error')) { " - $($r.Error)" })"
        return $false
    }
    return $true
}

function Send-TpStatus {
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [string] $Message = '',
        [string] $AlsoFile = ''
    )
    if (-not (Send-RelaySealed -Kind 'status' -Plaintext ([System.Text.Encoding]::UTF8.GetBytes($Body)))) {
        return $false
    }
    # A partial log from a cancelled run is the last thing the far side will
    # ever see of it, and losing it loses the only evidence there is.
    if ($AlsoFile -and (Test-Path -LiteralPath $AlsoFile -PathType Leaf)) {
        return (Send-RelaySealed -Kind 'log' -Plaintext ([System.IO.File]::ReadAllBytes($AlsoFile)))
    }
    return $true
}

function Send-TpProgress {
    param(
        [Parameter(Mandatory = $true)][string] $Body,
        [string] $Message = '',
        [string] $LogPath = ''
    )
    if (-not (Send-RelaySealed -Kind 'status' -Plaintext ([System.Text.Encoding]::UTF8.GetBytes($Body)))) {
        return $false
    }
    if (-not $LogPath -or -not (Test-Path -LiteralPath $LogPath -PathType Leaf)) { return $true }
    return (Send-RelaySealed -Kind 'progress' -Plaintext ([System.IO.File]::ReadAllBytes($LogPath)))
}

# A DISTINCT KIND from `progress`, and not a tidiness point. The kind travels
# inside the sealed envelope as a signed field, so the control side can tell a
# completed log from a mid-run snapshot without trusting the relay to label it.
# Sending the final log as `progress` would leave the reader unable to know it
# had the whole thing, which is the one question a log with a footer answers.
#
# The message is ignored: it is a git commit subject, and there is no history
# here to carry it.
function Send-TpLog {
    param(
        [Parameter(Mandatory = $true)][string] $LogPath,
        [string] $Message = ''
    )
    if (-not (Test-Path -LiteralPath $LogPath -PathType Leaf)) {
        Write-CapTpError "there is no log at $LogPath to deliver"
        return $false
    }
    return (Send-RelaySealed -Kind 'log' -Plaintext ([System.IO.File]::ReadAllBytes($LogPath)))
}

function Test-TpPreflight {
    $out = @()
    $out += @{ Status = 'ok'; Label = 'relay'; Detail = "$($script:Base), estate $($script:Estate), station $($script:Station)" }

    # THE FINGERPRINTS, BOTH OF THEM, and this is the line that saves a round
    # trip. Enrolment is the operator reading twelve characters aloud over a
    # different channel to confirm the other side is the other side; if this
    # table does not print them, that ceremony has nowhere to happen and the
    # residual risk in enrolment stays open.
    try {
        $out += @{ Status = 'ok'; Label = 'identity'; Detail = "$(Get-SealFingerprint $script:Identity)  <- read this to the control side" }
        $out += @{ Status = 'ok'; Label = 'peer'; Detail = "$(Get-SealFingerprint $script:Peer)  <- and check this against theirs" }
    } catch {
        $out += @{ Status = 'FAIL'; Label = 'identity'; Detail = "the key files will not parse: $($_.Exception.Message)" }
    }

    $out += @{ Status = 'ok'; Label = 'seal'; Detail = 'managed, in this payload. No binary to install and none to verify' }

    $seen = Get-RelayField 'seen'
    $sent = Get-RelayField 'out'
    if (Test-Path -LiteralPath $script:StatePath -PathType Leaf) {
        $out += @{ Status = 'ok'; Label = 'sequence'; Detail = "$($script:StatePath): accepted up to $seen, sent $sent" }
    } else {
        # NOT A FAILURE, and worth saying rather than leaving blank. A fresh
        # station legitimately has none; a station that has LOST one will accept
        # a replay of every message the relay still holds, and that is a
        # different thing wearing the same face.
        $out += @{ Status = 'warn'; Label = 'sequence'; Detail = "no state file at $($script:StatePath) yet, so every sequence number starts at zero. That is right on a first run and wrong after a restart - a lost state file lets the relay replay anything it still holds" }
    }
    return $out
}

Export-ModuleMember -Function @(
    'Get-TpCapabilities',
    'Initialize-Tp',
    'Get-TpScope',
    'Get-TpRevision',
    'Get-TpDescribe',
    'Test-Tp',
    'Send-TpLog',
    'Receive-TpRequest',
    'Receive-TpRequestLive',
    'Send-TpStatus',
    'Send-TpProgress',
    'Sync-TpSelf',
    'Test-TpPreflight'
)
