# No sensitive outputs, ever. An output is written to state and printed on
# apply, so a `sensitive = true` output is a secret in two more places.
output "site_url" {
  description = "Where the documentation site is served."
  value       = "https://${var.docs_hostname}"
}

output "legacy_site_url" {
  description = "The hostname that redirects to it, kept so a published link never breaks."
  value       = "https://${var.legacy_site_hostname}"
}

output "zone_id" {
  description = "The zone these records live in. An identifier."
  value       = data.cloudflare_zone.this.zone_id
}
