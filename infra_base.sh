#!/bin/bash
set -e

# Define variables
LOCATION="westeurope"
PROJECT_NAME="gofiopull250828"
PROJECT_NAME_NODASH="gofiopull250828"
REPO_NAME="viliamggh/go_fio_pull"

RESOURCE_GROUP_NAME="${PROJECT_NAME}-rg"
IDENTITY_NAME="${PROJECT_NAME}-uami"
STORAGE_ACCOUNT_NAME="${PROJECT_NAME_NODASH}st"
CONTAINER_NAME="terraform-state"
ACR_NAME="${PROJECT_NAME_NODASH}acr"

echo $STORAGE_ACCOUNT_NAME
echo $ACR_NAME
echo $IDENTITY_NAME
echo $RESOURCE_GROUP_NAME

# 1. Create a resource group
echo "Creating Resource Group: $RESOURCE_GROUP_NAME"
az group create --name "$RESOURCE_GROUP_NAME" --location "$LOCATION"

# 2. Create a user-assigned managed identity
echo "Creating User-Assigned Managed Identity: $IDENTITY_NAME"
IDENTITY_ID=$(az identity create --name "$IDENTITY_NAME" --resource-group "$RESOURCE_GROUP_NAME" --query id -o tsv)

sleep 20

# 3. Assign the user-assigned managed identity as the owner of the resource group
echo "Assigning User-Assigned Managed Identity to Resource Group: $RESOURCE_GROUP_NAME"
az role assignment create --assignee $(az identity show --name $IDENTITY_NAME --resource-group $RESOURCE_GROUP_NAME --query principalId -o tsv) --role "Owner" --scope "/subscriptions/$(az account show --query id -o tsv)/resourceGroups/$RESOURCE_GROUP_NAME"

# # 4. Create a storage account
echo "Creating Storage Account: $STORAGE_ACCOUNT_NAME"
az storage account create \
    --name "$STORAGE_ACCOUNT_NAME" \
    --resource-group "$RESOURCE_GROUP_NAME" \
    --location "$LOCATION" \
    --sku Standard_LRS \
    --kind StorageV2

# 5. Create a container within the storage account
echo "Creating Container: $CONTAINER_NAME"
az storage container create --name "$CONTAINER_NAME" --account-name "$STORAGE_ACCOUNT_NAME"

echo "All tasks completed successfully."

echo "Creating ACR: $ACR_NAME"
az acr create \
    --resource-group "$RESOURCE_GROUP_NAME" \
    --name "$ACR_NAME" \
    --sku Basic

sleep 20

az role assignment create --assignee $(az identity show --name $IDENTITY_NAME --resource-group $RESOURCE_GROUP_NAME --query clientId -o tsv) --role AcrPush --scope /subscriptions/$(az account show --query id -o tsv)/resourceGroups/$RESOURCE_GROUP_NAME/providers/Microsoft.ContainerRegistry/registries/$ACR_NAME

az identity federated-credential create \
  --resource-group $RESOURCE_GROUP_NAME \
  --identity-name $IDENTITY_NAME \
  --name gha-dev-env \
  --issuer https://token.actions.githubusercontent.com \
  --subject repo:${REPO_NAME}:environment:dev \
  --audiences api://AzureADTokenExchange

az identity federated-credential create \
  --resource-group $RESOURCE_GROUP_NAME \
  --identity-name $IDENTITY_NAME \
  --name gha-main-env \
  --issuer https://token.actions.githubusercontent.com \
  --subject repo:${REPO_NAME}:environment:main \
  --audiences api://AzureADTokenExchange


# Set env vars - need to run the below:
# export GH_TOKEN="<the‑token‑string>"
echo $(az identity show --name $IDENTITY_NAME --resource-group $RESOURCE_GROUP_NAME --query clientId -o tsv) | gh variable set UAMI_ID --repo $REPO_NAME
echo $(az identity show --name $IDENTITY_NAME --resource-group $RESOURCE_GROUP_NAME --query tenantId -o tsv) | gh variable set TENANT_ID --repo $REPO_NAME
echo $(az identity show --name $IDENTITY_NAME --resource-group $RESOURCE_GROUP_NAME | jq -r '.id | split("/")[2]') | gh variable set SUB_ID --repo $REPO_NAME
echo $ACR_NAME | gh variable set ACR_NAME --repo $REPO_NAME
echo $RESOURCE_GROUP_NAME | gh variable set RG_NAME --repo $REPO_NAME
echo $IDENTITY_NAME | gh variable set IDENTITY_NAME --repo $REPO_NAME