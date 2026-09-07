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
export TF_VAR_cloudflare_api_token="$(op read 'op://ScentVerdict/Cloudflare/api_token')"
export TF_VAR_github_token="$(gh auth token)"

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

## The gate

`.github/workflows/validate.yml` scans every commit for credential shapes and
for terraform state. It is not decoration: the whole argument above rests on
nothing secret being here, and an argument nobody checks is a hope.
