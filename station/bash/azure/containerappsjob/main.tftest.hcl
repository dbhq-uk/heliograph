# =============================================================================
#  The refusals, run offline
# =============================================================================
# See aci/main.tftest.hcl for why mock_provider is what makes a precondition
# testable at all: `terraform validate` never evaluates one, and `plan` against
# the real provider needs a subscription.
# `override_data` because the generated mock value for a resource id is a random
# string, and azurerm parses ids: without this the plan fails on "the number of
# segments didn't match" rather than on anything this file is asking about.
mock_provider "azurerm" {
  override_data {
    target = data.azurerm_container_app_environment.this
    values = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.App/managedEnvironments/cae"
    }
  }
}

variables {
  location                     = "westeurope"
  resource_group_name          = "rg"
  containerAppsEnvironmentName = "cae"
  repoUrl                      = "https://example.invalid/transport.git"
}

run "a_git_station_without_a_repo_url_is_refused" {
  command = plan
  variables {
    repoUrl = ""
  }
  expect_failures = [azurerm_container_app_job.this]
}

# A Container Apps secret name is lowercased with underscores as dashes, and
# that mapping is neither injective nor always valid. Each of these deploys
# something broken if it is not refused here.
run "two_keys_that_collide_once_lowercased_are_refused" {
  command = plan
  variables {
    extraSecureEnv = { TOKEN = "a", token = "b" }
  }
  expect_failures = [azurerm_container_app_job.this]
}

run "a_leading_underscore_makes_an_invalid_secret_name" {
  command = plan
  variables {
    extraSecureEnv = { _TOKEN = "a" }
  }
  expect_failures = [azurerm_container_app_job.this]
}

run "a_trailing_underscore_does_too" {
  command = plan
  variables {
    extraSecureEnv = { TOKEN_ = "a" }
  }
  expect_failures = [azurerm_container_app_job.this]
}

# GIT_TOKEN maps to git-token, which the template already creates for the
# gitToken variable. Terraform would emit two secrets with one name.
run "git_token_collides_with_the_built_in_secret" {
  command = plan
  variables {
    extraSecureEnv = { GIT_TOKEN = "a" }
  }
  expect_failures = [azurerm_container_app_job.this]
}

# AND ORDINARY KEYS ARE NOT REFUSED, or the checks above are just refusing
# everything - which is the way a precondition usually goes wrong.
run "ordinary_keys_are_accepted" {
  command = plan
  variables {
    transport      = "relay"
    repoUrl        = ""
    extraEnv       = { RELAY_URL = "https://relay.invalid", RELAY_ESTATE = "e", RELAY_STATION = "s" }
    extraSecureEnv = { RELAY_TOKEN = "t" }
  }
}
