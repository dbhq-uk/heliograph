data "cloudflare_zone" "this" {
  filter = { name = var.zone_name }
}

# The hostname the documentation site used to be served from.
#
# It is a redirect now, not a site. `heliograph.dbhq.uk` served the docs from
# GitHub Pages until 2026-09-16; they moved to `docs.heliograph.io` and this
# name became a 301, root to the apex and every deep path to the matching docs
# page. A proxied `100::` is Cloudflare's documented discard address: nothing
# routes to an origin, and the zone's dynamic redirect rule runs instead.
#
# THIS BLOCK DESCRIBED THE OLD WORLD UNTIL 2026-09-16, AND AN APPLY WOULD HAVE
# BROKEN THE LIVE ONE. It declared a DNS-only CNAME to `dbhq-uk.github.io`, and
# the live record had been an AAAA `100::`, proxied, since the day before. A
# plan would have shown one record to replace and read as routine; applying it
# would have deleted every published link's redirect and pointed the old name at
# a Pages site that no longer answers for it.
#
# The comment the old block carried is kept, because the constraint it records
# is still true and still decides what may be proxied: "GitHub Pages cannot
# complete its certificate challenge through Cloudflare's proxy, so a proxied
# record means Pages never issues a certificate and the site serves a warning
# forever." That is why `docs.heliograph.io` is DNS-only in the heliograph.io
# zone, and why this record could only be proxied after Pages stopped owning
# the hostname.
resource "cloudflare_dns_record" "legacy_site_redirect" {
  zone_id = data.cloudflare_zone.this.zone_id
  name    = var.legacy_site_hostname
  type    = "AAAA"
  content = "100::"
  proxied = true
  ttl     = 1

  comment = "Redirect only. 301 to heliograph.io; 100:: is the discard prefix"
}

# THE GITHUB PAGES BINDING IS GONE, and so is Pages.
#
# `github_repository_pages.site` lived here until 2026-09-16. It bound the
# repository to `docs.heliograph.io`, alongside `.github/workflows/pages.yml`
# which wrote a CNAME file on every deploy.
#
# What replaced it: a `heliograph-docs` Worker with static assets, deployed
# from the private cloud repository on a schedule. heliograph-cloud#62.
#
# **The reason is the transfer, not a preference.** A Pages custom domain is
# matched against the account that owns the repository, so moving `heliograph`
# to the `heliograph-io` organisation would have taken the documentation site
# down. Git redirects survive a transfer; a Pages custom domain does not. That
# coupling is what made the transfer a blocker at all (heliograph-cloud#59),
# and removing it is what unblocks it.
#
# Two things Pages could not do, which the Worker now does: serve a real 301
# for a moved page, and set a response header. `/intercom` was an HTML stub
# with a meta refresh because Pages does not read the generator's `_redirects`
# file, and no page carried `X-Content-Type-Options` at all - which matters on
# a site that serves markdown mirrors a browser could otherwise sniff as HTML.
#
# `docs.heliograph.io` in the heliograph.io zone is now a proxied AAAA at
# `100::`, created and owned by the Worker's custom domain rather than declared
# anywhere. The constraint recorded above is why it could not have been proxied
# before: Pages cannot complete its certificate challenge through Cloudflare's
# proxy, so a proxied record meant Pages never issued a certificate.

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
#
#   the redirect rule that makes heliograph.dbhq.uk a 301
#     A rule in the dbhq.uk zone's default http_request_dynamic_redirect
#     ruleset, created by hand on 2026-09-15. The record above is only half of
#     that redirect: without the rule, a proxied 100:: is a hostname that
#     answers and serves nothing. Worth bringing in here, and it is listed as
#     unmanaged rather than quietly assumed, because "the DNS is in terraform"
#     would otherwise read as "the redirect is in terraform".
#
#   docs.heliograph.io
#     A different zone, managed with the rest of heliograph.io. Only the Pages
#     binding for it is here, because the binding belongs to this repository.
