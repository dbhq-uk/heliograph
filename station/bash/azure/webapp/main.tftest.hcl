# See aci/main.tftest.hcl for why mock_provider is what makes a precondition
# testable at all.
# `override_data` because the generated mock value for a resource id is a random
# string, and azurerm parses ids: without this the plan fails on "the number of
# segments didn't match" rather than on anything this file is asking about.
mock_provider "azurerm" {
  override_data {
    target = data.azurerm_service_plan.this
    values = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Web/serverFarms/plan"
    }
  }
  override_data {
    target = data.azurerm_subnet.this
    values = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet"
    }
  }
}

variables {
  name                = "app-heliograph"
  location            = "westeurope"
  resource_group_name = "rg"
  vnetName            = "vnet"
  subnetName          = "snet"
  planName            = "plan"
}

run "a_git_station_without_a_repo_url_is_refused" {
  command = plan
  variables {
    repoUrl = ""
  }
  expect_failures = [azurerm_linux_web_app.this]
}

run "another_transport_needs_no_repo_url" {
  command = plan
  variables {
    transport = "blob"
    repoUrl   = ""
    extraEnv  = { PIGEONHOLE_ACCOUNT = "acct", PIGEONHOLE_LANE = "lane" }
  }
}
