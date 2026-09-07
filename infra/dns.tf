data "cloudflare_zone" "this" {
  filter = { name = var.zone_name }
}

# The documentation site.
#
# DNS-ONLY, not proxied, and that is not an oversight. GitHub Pages cannot
# complete its certificate challenge through Cloudflare's proxy, so a proxied
# record means Pages never issues a certificate and the site serves a warning
# forever. It can be proxied later, once the certificate exists.
resource "cloudflare_dns_record" "site" {
  zone_id = data.cloudflare_zone.this.zone_id
  name    = var.site_hostname
  type    = "CNAME"
  content = var.pages_target
  proxied = false
  ttl     = 1

  comment = "heliograph docs site, GitHub Pages. DNS-only so GitHub can issue its certificate."
}

# The GitHub Pages binding, so the CNAME and the repository agree.
#
# They are two halves of one fact, and setting only one of them is the failure
# that produces a site serving somebody else's 404.
resource "github_repository_pages" "site" {
  repository = "heliograph"
  cname      = var.site_hostname

  # WORKFLOW, NOT LEGACY, and a `source` block must not appear here.
  #
  # The site is built and deployed by .github/workflows/pages.yml. Declaring a
  # source branch flips build_type to "legacy", which makes Pages serve the
  # repository root instead - so the first apply would have taken the live site
  # down and served the raw markdown. The plan said "2 to change" and looked
  # harmless.
  build_type = "workflow"
}

# NOT MANAGED HERE, and each for a reason:
#
#   relay.heliograph.dbhq.uk
#     Created by wrangler from the `routes` block in heliograph-relay's
#     wrangler.toml. That is already declarative and already committed, and
#     splitting a DNS record from the Worker it points at would mean two tools
#     could disagree about whether the relay exists.
#
#   the advanced certificate pack for relay.heliograph.dbhq.uk
#     Cloudflare creates it automatically when the custom domain is added. A
#     four-label host is not covered by Universal SSL, which only reaches
#     *.dbhq.uk. Managing it here would fight the automatic one.
#
#   the workers.dev subdomain
#     A one-time, account-wide setting. Terraform would own an account-level
#     resource for the sake of a single string that can never change.
#
#   estate tokens
#     Secrets, held as wrangler secrets and in 1Password. They must not enter
#     terraform state, which is the strongest argument for keeping them out of
#     terraform entirely.
