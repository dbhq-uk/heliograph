# The public surface of heliograph: DNS, and the GitHub Pages binding.
#
# WHAT IS AND IS NOT A SECRET, because the distinction decides what may live in
# a public repository and getting it wrong in either direction is expensive.
#
#   NOT SECRET, and deliberately committed:
#     account id, zone id, hostnames, record contents. These are identifiers.
#     They appear in every API call the account makes, Cloudflare's own
#     documentation uses them in examples, and Paseo publishes theirs. Treating
#     an identifier as a secret buys nothing and costs the reader the ability to
#     understand what is deployed.
#
#   SECRET, and never in this repository in any form:
#     API tokens. Supplied as TF_VAR_* at apply time, from 1Password.
#
#   THE ACTUAL RISK, which is neither of those:
#     terraform state. It stores resource attributes verbatim, and for some
#     resources that includes secret values. State must never be in git, public
#     OR private. It lives in R2, below.
terraform {
  required_version = ">= 1.6"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }

  # State in Cloudflare R2, which is S3-compatible, so the standard backend
  # works and no new vendor is involved.
  #
  # Deliberately NOT in git. A private repository would be the obvious answer
  # and it is the wrong one: state is written on every apply, it contains
  # values that were never meant to be read, and a repository is a thing people
  # clone. R2 keeps it in one place with one access path.
  #
  # Configured at init from backend.hcl, which is committed because every value
  # in it is an identifier rather than a credential. The keys come from
  # 1Password by way of ~/.dbhq/env.sh and never touch disk.
  #
  #   terraform init -backend-config=backend.hcl
  backend "s3" {}
}

provider "cloudflare" {
  # From TF_VAR_cloudflare_api_token. Never a default, and never written here.
  api_token = var.cloudflare_api_token
}

provider "github" {
  owner = "dbhq-uk"
  token = var.github_token
}
