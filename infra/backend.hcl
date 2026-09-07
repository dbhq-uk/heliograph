# Backend configuration for the R2 state bucket.
#
# Committed on purpose. Every value here is an identifier, not a credential:
# a bucket name and an account-scoped endpoint. The keys that open it come from
# 1Password at init time, as AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, and
# are never written to disk.
#
# This file exists because the alternative was nine -backend-config flags typed
# by hand. A nine-flag command is one somebody eventually gets wrong, and
# getting it wrong here means terraform silently initialises a DIFFERENT state
# file and then plans to create infrastructure that already exists.
#
#   terraform init -backend-config=backend.hcl

bucket = "heliograph-tfstate"
key    = "public-surface.tfstate"

endpoints = { s3 = "https://691c21cdcf1b3fa4add70cc166e99733.r2.cloudflarestorage.com" }

# R2 has no regions, but the S3 client refuses to start without one. `auto` is
# what Cloudflare document.
region = "auto"

# R2 is S3-compatible, not S3. Each of these switches off a check that assumes
# a real AWS endpoint on the other end, and each one fails the init without it.
skip_credentials_validation = true
skip_region_validation      = true
skip_requesting_account_id  = true
skip_s3_checksum            = true
use_path_style              = true
