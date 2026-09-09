# =============================================================================
#  The refusals, run offline
# =============================================================================
# A PRECONDITION NOBODY HAS WATCHED FAIL IS A PRECONDITION NOBODY KNOWS WORKS,
# and `terraform validate` never evaluates one - it checks that the
# configuration is well formed, not what it does with values. `terraform plan`
# would, and it needs a subscription: this template reads a real subnet before
# it reaches the resource.
#
# `mock_provider` is what makes them testable at all. It stands in for azurerm
# entirely, so a plan runs with no credentials, no network and no resources,
# and the preconditions are evaluated for real.
# `override_data` because the generated mock value for a subnet id is a random
# string, and azurerm parses that id: the plan fails on "the number of segments
# didn't match" rather than on anything this file is asking about.
mock_provider "azurerm" {
  override_data {
    target = data.azurerm_subnet.this
    values = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet"
    }
  }
}

variables {
  location            = "westeurope"
  resource_group_name = "rg"
  vnetName            = "vnet"
  subnetName          = "snet"
}

# THE DEFAULT CONFIGURATION USED TO DEPLOY AND THEN EXIT 2. Making repoUrl
# optional for the transports that have nothing to clone also made it optional
# for git, where the entrypoint cannot do without it: the container came up,
# printed "no repository URL given", and stopped - after terraform reported
# success.
run "a_git_station_without_a_repo_url_is_refused" {
  command = plan
  variables {
    repoUrl = ""
  }
  expect_failures = [azurerm_container_group.this]
}

run "and_with_one_it_plans" {
  command = plan
  variables {
    repoUrl = "https://example.invalid/transport.git"
  }
}

# The other transports have no repository, and requiring one would be the same
# defect in the other direction.
run "another_transport_needs_no_repo_url" {
  command = plan
  variables {
    transport      = "blob"
    repoUrl        = ""
    extraEnv       = { PIGEONHOLE_ACCOUNT = "acct", PIGEONHOLE_LANE = "lane" }
    extraSecureEnv = { PIGEONHOLE_SAS = "sv=x" }
  }
}
