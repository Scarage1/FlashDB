#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# FlashDB — Azure Container Apps Setup (Free Tier)
# ─────────────────────────────────────────────────────────────────────────────
# Prerequisites: Azure CLI installed & logged in (az login)
#
# Usage:
#   chmod +x deploy/azure-setup.sh
#   ./deploy/azure-setup.sh
#
# What this creates (all free tier / consumption-based):
#   • Resource Group
#   • Log Analytics Workspace (required by Container Apps)
#   • Container Apps Environment
#   • Container App (from GHCR image)
#
# Free tier includes:
#   • 2 million requests/month
#   • 180,000 vCPU-seconds/month
#   • 360,000 GiB-seconds/month
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
RESOURCE_GROUP="${AZURE_RESOURCE_GROUP:-flashdb-rg}"
LOCATION="${AZURE_LOCATION:-eastus}"
CONTAINER_ENV="${AZURE_CONTAINER_ENV:-flashdb-env}"
APP_NAME="${AZURE_APP_NAME:-flashdb}"
IMAGE="${GHCR_IMAGE:-ghcr.io/scarage1/flashdb:master}"
LOG_ANALYTICS="${AZURE_LOG_ANALYTICS:-flashdb-logs}"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           FlashDB — Azure Container Apps Setup              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  Resource Group:    $RESOURCE_GROUP"
echo "  Location:          $LOCATION"
echo "  Container App:     $APP_NAME"
echo "  Image:             $IMAGE"
echo ""

# ── 1. Resource Group ────────────────────────────────────────────────────────
echo "▸ Creating resource group..."
az group create \
  --name "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  --output none

# ── 2. Log Analytics Workspace ───────────────────────────────────────────────
echo "▸ Creating Log Analytics workspace..."
az monitor log-analytics workspace create \
  --resource-group "$RESOURCE_GROUP" \
  --workspace-name "$LOG_ANALYTICS" \
  --location "$LOCATION" \
  --output none

LOG_ANALYTICS_ID=$(az monitor log-analytics workspace show \
  --resource-group "$RESOURCE_GROUP" \
  --workspace-name "$LOG_ANALYTICS" \
  --query customerId -o tsv)

LOG_ANALYTICS_KEY=$(az monitor log-analytics workspace get-shared-keys \
  --resource-group "$RESOURCE_GROUP" \
  --workspace-name "$LOG_ANALYTICS" \
  --query primarySharedKey -o tsv)

# ── 3. Container Apps Environment ────────────────────────────────────────────
echo "▸ Creating Container Apps environment (consumption/free tier)..."
az containerapp env create \
  --name "$CONTAINER_ENV" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  --logs-workspace-id "$LOG_ANALYTICS_ID" \
  --logs-workspace-key "$LOG_ANALYTICS_KEY" \
  --output none

# ── 4. Container App ─────────────────────────────────────────────────────────
echo "▸ Deploying FlashDB container app..."
az containerapp create \
  --name "$APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$CONTAINER_ENV" \
  --image "$IMAGE" \
  --target-port 8080 \
  --ingress external \
  --cpu 0.25 \
  --memory 0.5Gi \
  --min-replicas 0 \
  --max-replicas 1 \
  --env-vars \
    FLASHDB_ADDR=:6379 \
    FLASHDB_DATA=/data \
  --output none

# ── 5. Get the URL ───────────────────────────────────────────────────────────
FQDN=$(az containerapp show \
  --name "$APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query properties.configuration.ingress.fqdn -o tsv)

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  ✅  FlashDB deployed successfully!                         ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  🌐 Web UI:  https://$FQDN"
echo "  📡 API:     https://$FQDN/api/v1/stats"
echo "  🏥 Health:  https://$FQDN/healthz"
echo ""
echo "  ⚠️  RESP (port 6379) is NOT exposed via HTTP ingress."
echo "     For Redis-compatible access, use Azure VNet or SSH tunnel."
echo ""
echo "  💡 To set an API token:"
echo "     az containerapp update --name $APP_NAME \\"
echo "       --resource-group $RESOURCE_GROUP \\"
echo "       --set-env-vars FLASHDB_API_TOKEN=your-secret-token"
echo ""
echo "  🧹 To tear down everything:"
echo "     az group delete --name $RESOURCE_GROUP --yes --no-wait"
echo ""
