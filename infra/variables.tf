# Secrets have NO default. That is the whole mechanism: terraform refuses to
# plan without them, so there is no path where a placeholder gets committed and
# quietly used.
variable "cloudflare_api_token" {
  description = "Cloudflare API token. Supply as TF_VAR_cloudflare_api_token, from 1Password. Never commit."
  type        = string
  sensitive   = true
}

variable "github_token" {
  description = "GitHub token with repo admin, for the Pages custom domain. Supply as TF_VAR_github_token."
  type        = string
  sensitive   = true
}

# Identifiers, not secrets. Committed on purpose: a reader should be able to see
# exactly what is deployed and where, which is the same argument that keeps the
# relay's source public.
variable "cloudflare_account_id" {
  description = "Cloudflare account id. An identifier, not a credential."
  type        = string
  default     = "691c21cdcf1b3fa4add70cc166e99733"
}

variable "zone_name" {
  description = "The DNS zone these records live in."
  type        = string
  default     = "dbhq.uk"
}

variable "site_hostname" {
  description = "Where the documentation site is served."
  type        = string
  default     = "heliograph.dbhq.uk"
}

variable "pages_target" {
  description = "The GitHub Pages host the site CNAME points at."
  type        = string
  default     = "dbhq-uk.github.io"
}
