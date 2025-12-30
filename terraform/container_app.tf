resource "azurerm_container_app" "bank_pull" {
  name                         = "${var.project_name_no_dash}aca"
  container_app_environment_id = azurerm_container_app_environment.c_app_env.id
  resource_group_name          = data.azurerm_resource_group.app_rg.name
  revision_mode                = "Single"

  identity {
    type         = "UserAssigned"
    identity_ids = [local.app_identity_id]
  }
  # identity {
  #   type = "SystemAssigned"
  # }
  lifecycle {
    replace_triggered_by = [
      null_resource.always_run
    ]
  }
  ingress {
    external_enabled           = true
    allow_insecure_connections = true
    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
    target_port = 8080
    # ip_security_restriction {
    #   name             = "my_machine"
    #   action           = "Allow"
    #   ip_address_range = "109.81.89.47"
    # }
  }
  registry {
    server   = local.acr_login_server
    identity = local.app_identity_id
  }
  template {
    container {
      name   = replace(var.image_name, "_", "")
      image  = "${local.acr_login_server}/${var.image_name}:${var.environment}"
      cpu    = 0.25
      memory = "0.5Gi"

      liveness_probe {
        transport = "HTTP"
        path      = "/health"
        port      = 8080
      }

      readiness_probe {
        transport = "HTTP"
        path      = "/health"
        port      = 8080
      }

      env {
        name  = "AZURE_CLIENT_ID"
        value = local.app_identity_client_id
      }

      env {
        name  = "AZURE_TENANT_ID"
        value = local.app_identity_tenant_id
      }

      env {
        name  = "STORAGE_ACCOUNT_URL"
        value = local.storage_account_url
      }

      env {
        name  = "KEY_VAULT_URL"
        value = local.key_vault_url
      }

      env {
        name  = "STORAGE_CONTAINER_NAME"
        value = local.storage_container_name
      }

      env {
        name  = "ACCOUNT_ALIASES"
        value = var.account_aliases
      }
    }
  }

}