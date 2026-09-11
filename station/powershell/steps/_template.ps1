# =============================================================================
#  _template.ps1 - copy this to start a step
# =============================================================================
# heliograph-mode: read-only     # the runner refuses a step that declares
#                                # nothing. Use `action` for a step that
#                                # changes state; it then also needs
#                                # CONFIRM=yes in the request's env line AND a
#                                # station started with --allow-actions.
#
#  A STEP PRINTS TO STDOUT AND NOTHING ELSE.
#
#  It does not open the log, write timestamps, publish, or deliver. run.ps1 and
#  caplib.psm1 own all of that, which is what lets you run this file straight to
#  a terminal while you are writing it:
#
#      pwsh -File .\steps\my-step.ps1
#
#  Wire it up by adding one line to run.ps1's table:
#
#      $StepTable['mystep'] = 'steps/my-step.ps1'
#
#  THE TABLE IS ORDINAL - case-sensitive - because run.sh's `case` is, and a
#  step name that resolves on one implementation and is unknown on the other is
#  the kind of difference that makes a twin useless.
#
#  RULES, all of them paid for by an investigation that went wrong first:
#
#    NEVER PROMPT. No Read-Host, no -Confirm, no credential dialog. A prompt
#    through the capture pipeline is invisible and the run just hangs until
#    somebody notices, which is a wasted round trip through an operator who
#    cannot see it either.
#
#    NEVER BUFFER OUTPUT AND PRINT IT AT THE END. Every line is timestamped
#    when it is produced, so collecting into a variable and writing it in one
#    go gives you a log where every line carries the same time - which reads
#    like a working log while destroying the only property that makes these
#    logs worth having.
#
#    SAVE THIS FILE AS UTF-8 WITHOUT A BOM. Windows editors add one by default.
#    A BOM is three invisible bytes in front of the first character, so the
#    `# heliograph-mode:` line stops being the first thing on its line and the
#    step is refused as undeclared. run.ps1 refuses a BOM outright and says so,
#    because "your editor added three invisible bytes" is a fixable answer and
#    "declares no mode" is not.
#
#    EXIT CODES ARE YOURS. Anything non-zero is reported honestly and the log
#    still ships: a failed step is evidence, not an error to be hidden. The
#    runner passes your code through unchanged.
#
#    DO NOT REDACT ANYTHING YOURSELF. caplib.psm1 masks secrets on the way into
#    the log, line by line. A step that masks its own output hides what the
#    redactor would have caught and proves nothing about whether it works.
# =============================================================================

Write-Output "host      : $([System.Net.Dns]::GetHostName())"
Write-Output "user      : $([System.Environment]::UserName)"
Write-Output "powershell: $($PSVersionTable.PSVersion)"

# Native commands work exactly as they do at a prompt. Their stdout and stderr
# are merged in the child, in order, so an error lands between the lines that
# came before and after it rather than at the end.
& hostname
