# =============================================================================
#  lib/cancel.psm1 - a cancel that takes the whole tree with it
# =============================================================================
# `cancel: yes` has to stop the STEP, not the wrapper that launched it. A step
# runs `terraform`, which runs a provider, which runs `git`; signalling only the
# thing the station started leaves all of that running, and the operator is told
# the run was cancelled while it carries on changing the estate.
#
# The bash station gets this from `setsid` plus a negative pid: the step is a
# process GROUP leader, and one kill reaches the group. Windows has no process
# groups. It has JOB OBJECTS, which are better for this: a job with
# JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE terminates every process in it the moment
# the last handle closes, and a child of a job is in the job. So killing the
# station's own process takes the entire tree with it, including anything that
# started after the kill was decided.
#
# THREE STRATEGIES, AND THE FALLBACK IS NOT COSMETIC:
#
#   Windows, Add-Type available    a Job Object. Nothing can escape it, and no
#                                  cleanup is needed - the kernel does it.
#   Windows, Add-Type BLOCKED      `taskkill /T /F`, which walks the child
#                                  tree. A hardened estate can and does block
#                                  Add-Type, by Constrained Language Mode or by
#                                  policy, and that estate is exactly the one
#                                  this whole PowerShell station exists for.
#                                  A cancel that only worked where Add-Type is
#                                  permitted would be missing where it is
#                                  needed.
#   anything else                  setsid's process group, the way run.sh does.
#
# `taskkill /T` walks the tree by parent id AT THE MOMENT IT RUNS, so a
# grandchild started a millisecond later survives it. That is a real gap and it
# is why the Job Object is preferred rather than merely tidier. Both are used
# together where both are available: the job guarantees the outcome, taskkill
# makes it prompt.
# =============================================================================

Set-StrictMode -Version 2.0

function Test-CancelWindows {
    # 5.1 does not define $IsWindows at all, and under Set-StrictMode reading an
    # undefined variable is an error rather than $false.
    return [System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT
}

$script:JobHandle = [IntPtr]::Zero
$script:JobReady = $false

function Enter-CapKillGroup {
    <#
      .SYNOPSIS
      Put THIS process, and therefore everything it starts, into a group that
      dies together. Returns the strategy actually in force, as a string.

      .DESCRIPTION
      Called once, early, by anything that will start a step. After it returns,
      killing this process is enough - no bookkeeping of child pids, and no
      window in which a child started after the kill decision escapes.
    #>
    if (-not (Test-CancelWindows)) {
        # Nothing to do. The caller is started under `setsid` and a kill to the
        # negative pid reaches the group, which is run.sh's arrangement.
        return 'process-group'
    }

    try {
        if (-not $script:JobReady) {
            # P/Invoke, compiled once per process. THIS IS THE CALL A HARDENED
            # ESTATE BLOCKS: Constrained Language Mode refuses Add-Type
            # outright, and some policies refuse the compiler. The catch below
            # is the whole reason the fallback exists.
            if (-not ([System.Management.Automation.PSTypeName]'Heliograph.Job').Type) {
                Add-Type -ErrorAction Stop -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace Heliograph {
  public static class Job {
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern IntPtr CreateJobObject(IntPtr a, string name);

    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool SetInformationJobObject(
        IntPtr job, int infoClass, IntPtr info, uint infoLength);

    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);

    [StructLayout(LayoutKind.Sequential)]
    public struct JOBOBJECT_BASIC_LIMIT_INFORMATION {
      public Int64 PerProcessUserTimeLimit;
      public Int64 PerJobUserTimeLimit;
      public UInt32 LimitFlags;
      public UIntPtr MinimumWorkingSetSize;
      public UIntPtr MaximumWorkingSetSize;
      public UInt32 ActiveProcessLimit;
      public Int64 Affinity;
      public UInt32 PriorityClass;
      public UInt32 SchedulingClass;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct IO_COUNTERS {
      public UInt64 ReadOperationCount;
      public UInt64 WriteOperationCount;
      public UInt64 OtherOperationCount;
      public UInt64 ReadTransferCount;
      public UInt64 WriteTransferCount;
      public UInt64 OtherTransferCount;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION {
      public JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
      public IO_COUNTERS IoInfo;
      public UIntPtr ProcessMemoryLimit;
      public UIntPtr JobMemoryLimit;
      public UIntPtr PeakProcessMemoryUsed;
      public UIntPtr PeakJobMemoryUsed;
    }
  }
}
'@
            }

            $h = [Heliograph.Job]::CreateJobObject([IntPtr]::Zero, $null)
            if ($h -eq [IntPtr]::Zero) { throw 'CreateJobObject returned NULL' }

            $info = New-Object Heliograph.Job+JOBOBJECT_EXTENDED_LIMIT_INFORMATION
            # 0x2000 = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE. The one flag that
            # matters: every process in the job is terminated when the last
            # handle to it closes, which happens when this process dies for any
            # reason at all - including one it never got to handle.
            $info.BasicLimitInformation.LimitFlags = 0x2000

            $size = [System.Runtime.InteropServices.Marshal]::SizeOf($info)
            $ptr = [System.Runtime.InteropServices.Marshal]::AllocHGlobal($size)
            try {
                [System.Runtime.InteropServices.Marshal]::StructureToPtr($info, $ptr, $false)
                # 9 = JobObjectExtendedLimitInformation
                if (-not [Heliograph.Job]::SetInformationJobObject($h, 9, $ptr, [uint32]$size)) {
                    throw 'SetInformationJobObject failed'
                }
            } finally {
                [System.Runtime.InteropServices.Marshal]::FreeHGlobal($ptr)
            }

            $me = [System.Diagnostics.Process]::GetCurrentProcess().Handle
            if (-not [Heliograph.Job]::AssignProcessToJobObject($h, $me)) {
                throw 'AssignProcessToJobObject failed'
            }

            # HELD, deliberately, for the life of the process. Closing it would
            # kill us and everything we started, which is precisely the
            # behaviour being bought - so the handle is kept and never disposed.
            $script:JobHandle = $h
            $script:JobReady = $true
        }
        return 'job-object'
    } catch {
        # NOT FATAL, and this is the case the fallback exists for. An estate
        # that blocks Add-Type is the estate with no bash, which is the estate
        # this station is for. A cancel that walks the tree is worse than a job
        # object and enormously better than none.
        return 'taskkill'
    }
}

function Stop-CapTree {
    <#
      .SYNOPSIS
      Kill a process and everything under it. Returns $true when it is gone.

      .DESCRIPTION
      Belt and braces where both are available: `taskkill /T /F` makes it
      prompt, and if the target put itself in a job object then its death
      closes the last handle and the kernel takes the rest - including anything
      started after taskkill enumerated the tree.
    #>
    param(
        [Parameter(Mandatory = $true)][int] $ProcessId,
        [int] $TimeoutSeconds = 10
    )

    if (Test-CancelWindows) {
        & taskkill /T /F /PID $ProcessId 2>&1 | Out-Null
    } else {
        # The GROUP, negated, the way run.sh cancels - or the step survives its
        # wrapper. `kill` is not a PowerShell cmdlet here; the external one is
        # what understands a negative pid.
        & kill -TERM -- "-$ProcessId" 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { & kill -TERM $ProcessId 2>&1 | Out-Null }
    }

    # PROVED GONE, not merely signalled. A kill that was sent is not a kill that
    # landed, and a caller that carries on regardless is racing a process still
    # writing to the log it is about to read.
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        if (-not (Test-CapAlive -ProcessId $ProcessId)) { return $true }
        Start-Sleep -Milliseconds 100
    }

    if (-not (Test-CancelWindows)) {
        & kill -KILL -- "-$ProcessId" 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { & kill -KILL $ProcessId 2>&1 | Out-Null }
        Start-Sleep -Milliseconds 500
    }
    return (-not (Test-CapAlive -ProcessId $ProcessId))
}

function Test-CapAlive {
    param([Parameter(Mandatory = $true)][int] $ProcessId)
    try {
        $p = Get-Process -Id $ProcessId -ErrorAction Stop
        # A ZOMBIE IS NOT ALIVE. On Unix a killed child stays in the table
        # until its parent reaps it, and Get-Process can still find it - so
        # "the process exists" would be true for ever and Stop-CapTree would
        # report failure on a cancel that worked perfectly.
        #
        # UNTESTED, and said so rather than left to look covered.
        # PowerShell's Start-Process reaps its own children, so the test suite
        # cannot construct the case; an assertion that was written for it
        # passed with this line deleted, which is worse than no assertion at
        # all. It stays because the cost is one comparison and the failure it
        # prevents is a cancel reported as failed.
        if ($p.HasExited) { return $false }
        return $true
    } catch {
        return $false
    }
}

Export-ModuleMember -Function @(
    'Enter-CapKillGroup',
    'Stop-CapTree',
    'Test-CapAlive',
    'Test-CancelWindows'
)
