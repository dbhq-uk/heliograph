# heliograph compared with AWS SSM Run Command and Azure Run Command

If you can use Run Command, use it. This page is for the estate where you
cannot, and for the engineer who wants to know what the difference actually is
before adopting a tool nobody has heard of.

## The short version

| | AWS Systems Manager Run Command | Azure Run Command | heliograph |
|---|---|---|---|
| on the target | SSM Agent, registered with your account | the Azure VM agent; on other machines, the Arc Connected Machine agent | nothing installed: bash 4+, git, GNU coreutils |
| who must own the machine | your AWS account, or a hybrid activation you created | your Azure subscription, or an Arc-enabled server in it | anyone. An operator plants a directory and starts a loop |
| who runs the command | the agent, as root on Linux | an elevated user, by default | the operator's own account; root is refused |
| what comes back | the first 24,000 characters of stdout through the API; the rest only if you sent it to S3 or CloudWatch Logs first | the last 4,096 bytes; more only if streamed to a storage blob | the whole log, every line with a UTC timestamp, passed or failed |
| default posture | whatever the IAM policy allows | whatever the RBAC role allows | read-only. An action needs `CONFIRM=yes` and a station started with `--allow-actions` |
| a human in the loop | none needed | none needed | one, once: somebody starts the station |
| fleet | hundreds of nodes, by tag | one script at a time per VM | one step at a time per station |

The last two rows are the point. Run Command needs no human and scales to a
fleet, and it needs the machine to be yours: an agent you installed, an account
that owns it, a role you were granted. heliograph needs one human to run one
command, and in exchange it works on a machine that is not yours and never will
be.

## AWS Systems Manager Run Command

A managed node is an EC2 instance, or a non-EC2 machine registered through a
hybrid activation: install SSM Agent, create an IAM service role, register the
machine with the activation code. The agent needs outbound HTTPS to the Systems
Manager endpoints. The person sending the command needs `ssm:SendCommand`,
scoped by document and, if you are careful, by tag.

Output is where it stops being a log. `GetCommandInvocation` returns the first
24,000 characters of stdout and the first 8,000 of stderr; for the rest you
configure an S3 bucket or a CloudWatch log group before the run. The output is
not timestamped line by line, so a command that stalled for three minutes and
one that produced output steadily read the same afterwards.

From 30 September 2026, Run Command on hybrid managed nodes is priced per use
rather than through the retired advanced-instances tier. Check the current
pricing page before planning around it.

## Azure Run Command

The action form, `az vm run-command invoke`, runs a script through the VM
agent. One script at a time per VM, 90 minutes at most, scripts run as an
elevated user by default, and **output is limited to the last 4,096 bytes**. The
VM needs outbound port 443 to Azure to return results at all; the script can
succeed and the output never arrive. Running one needs
`Microsoft.Compute/virtualMachines/runCommands/write`, which Virtual Machine
Contributor carries.

The managed form, `az vm run-command create`, keeps the command as a resource
and can stream stdout and stderr to append blobs, which lifts the size limit at
the cost of a storage account and a SAS URI per run. For machines outside Azure,
Run Command on Azure Arc-enabled servers goes through the Connected Machine
agent; at the time of writing it is in preview and not in the portal.

## Where heliograph is different, and where it is worse

**Nothing to install, and no account over the target.** The station is a
directory of bash that the operator plants and starts. It holds no credentials
of its own, so what it can do is [what that account can do](/security), and
root is refused rather than warned about.

**The whole run comes back.** Every line carries a UTC timestamp, so a hang is a
gap you can measure with `heliograph logs --gaps`, and the log ships whether the
step passed or failed. There is no size cut-off to configure around before the
run you needed it for.

**Read-only until earned.** A step declares itself in its own file. Run Command
executes whatever the policy allows; heliograph refuses an action unless three
separate things say yes, and publishes the refusal so you learn in seconds
rather than after a round trip.

**Worse, honestly.** It needs a person to start the loop, it is one machine and
one step at a time, and it is a young tool. If your machines are in your cloud
account with the agent already running, Run Command is the right answer and
this page should have told you so by now.

## And SSH, Teleport, Boundary, Tailscale SSH

Those give you access. If you can have access, take it. heliograph is for the
estate where access is refused by policy rather than capability, and where a
tunnel would be a breach rather than a convenience. It does not tunnel, proxy or
hold a connection open, and there is nothing in it to punch through a firewall
with: [what it will not do](/security#it-does-not-give-you-access-you-do-not-have).
