# infra

The public surface of heliograph: DNS, and the GitHub Pages binding.

## What is and is not a secret

The distinction decides what may live in a public repository, and getting it
wrong in either direction is expensive.

| | |
|---|---|
| **Not secret, committed on purpose** | account id, zone id, hostnames, record contents |
| **Secret, never here in any form** | API tokens |
| **The actual risk** | terraform **state** |

Account and zone ids are **identifiers**. They appear in every API call the
account makes, Cloudflare's own documentation uses them in examples, and Paseo
publishes theirs. Treating an identifier as a secret buys nothing and costs the
reader the ability to see what is deployed - which, for a product whose claim is
that its relay cannot read your logs, is a cost worth refusing to pay.

**State is the part people get wrong.** It stores resource attributes verbatim,
and for some resources that includes secret values. A private repository is the
obvious answer and it is still the wrong one: state is rewritten on every apply,
and a repository is a thing people clone. It lives in R2 instead.

## Running it

Tokens come from 1Password at apply time. Nothing is written to disk.

```bash
set -a; . ~/.scentverdict/env.sh; set +a          # 1Password service account
export TF_VAR_cloudflare_api_token="$(op read 'op://DBHQ/Cloudflare/api_token')"
export TF_VAR_github_token="$(gh auth token)"
export AWS_ACCESS_KEY_ID="$(op read 'op://DBHQ/heliograph - R2 terraform state/access_key_id')"
export AWS_SECRET_ACCESS_KEY="$(op read 'op://DBHQ/heliograph - R2 terraform state/secret_access_key')"

terraform init \
  -backend-config="bucket=heliograph-tfstate" \
  -backend-config="key=public-surface.tfstate" \
  -backend-config="endpoints={s3=\"https://<account>.r2.cloudflarestorage.com\"}" \
  -backend-config="region=auto" \
  -backend-config="skip_credentials_validation=true" \
  -backend-config="skip_region_validation=true" \
  -backend-config="skip_requesting_account_id=true" \
  -backend-config="skip_s3_checksum=true" \
  -backend-config="use_path_style=true"

terraform plan
```

Secret variables have **no default**. That is the mechanism rather than a
convention: terraform refuses to plan without them, so there is no path where a
placeholder gets committed and quietly used.

## What is deliberately not managed here

| | why |
|---|---|
| `relay.heliograph.dbhq.uk` | wrangler creates it from `routes` in the relay's `wrangler.toml`. Already declarative, already committed, and splitting a DNS record from the Worker it points at means two tools can disagree about whether the relay exists |
| its certificate pack | Cloudflare creates it automatically. A four-label host is not covered by Universal SSL, which reaches only `*.dbhq.uk`. Managing it here would fight the automatic one |
| the `workers.dev` subdomain | a one-time account-wide setting, for a single string that can never change |
| estate tokens | secrets. Keeping them out of terraform is the point: they must never enter state |

## The DBHQ convention: R2 holds every project's state

Standing rule, not a heliograph decision. Every DBHQ project that uses terraform
keeps its state in Cloudflare R2, never in git and never only on a laptop.

**One bucket per project, named `<project>-tfstate`**, with its own R2 token
scoped to that bucket alone.

That last part is the whole reason for the shape, and it is worth knowing before
somebody tidies it into a single shared bucket: **R2 tokens scope to a bucket,
not to a prefix.** One bucket with `heliograph/`, `scentverdict/` and the rest
under it would mean one credential that reads every project's state, and state
holds secret values verbatim. Per-project buckets are the only way the isolation
is real rather than cosmetic.

Setting up a new project is two things in the dashboard and one paste:

1. R2 -> Create bucket -> `<project>-tfstate`
2. R2 -> Manage R2 API Tokens -> Object Read & Write, **scoped to that bucket**
3. Store as `op://DBHQ/<project> - R2 terraform state` with `access_key_id` and
   `secret_access_key`, then copy the `terraform init` block below and change
   the bucket name.

The endpoint and account id are the same for every project. Neither is a secret.

This belongs in a shared DBHQ infrastructure repository once one exists. It is
written here because it is true now and an unwritten convention is not one.

## Credentials

Held in 1Password, read at apply time, never written to disk.

| | |
|---|---|
| `DBHQ/Cloudflare/api_token` | the provider |
| `DBHQ/heliograph - R2 terraform state` | the state backend, scoped to that bucket alone |

The R2 credential is **Object Read & Write on `heliograph-tfstate` only**. It
cannot read another bucket, and it is not the Cloudflare API token: losing it
costs you the state file, not the account.

## The gate

`.github/workflows/validate.yml` scans every commit for credential shapes and
for terraform state. It is not decoration: the whole argument above rests on
nothing secret being here, and an argument nobody checks is a hope.
