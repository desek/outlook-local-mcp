#!/usr/bin/env bash
# apply-app-registration.sh
#
# Applies the master app registration definition to Azure AD using the
# Microsoft Graph API. Reads infra/app-registration.json and PATCHes
# the application object.
#
# Prerequisites:
#   - Azure CLI installed and logged in (az login)
#   - Sufficient permissions to update the app registration
#
# Usage:
#   ./infra/apply-app-registration.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JSON_FILE="${SCRIPT_DIR}/app-registration.json"

if [[ ! -f "$JSON_FILE" ]]; then
  echo "ERROR: $JSON_FILE not found" >&2
  exit 1
fi

# Read app and object IDs from the JSON file.
APP_ID=$(jq -r '._appId' "$JSON_FILE")
OBJECT_ID=$(jq -r '._objectId' "$JSON_FILE")

if [[ -z "$APP_ID" || "$APP_ID" == "null" ]]; then
  echo "ERROR: _appId not found in $JSON_FILE" >&2
  exit 1
fi
if [[ -z "$OBJECT_ID" || "$OBJECT_ID" == "null" ]]; then
  echo "ERROR: _objectId not found in $JSON_FILE" >&2
  exit 1
fi

echo "Applying app registration metadata..."
echo "  App ID:    $APP_ID"
echo "  Object ID: $OBJECT_ID"

# Build the PATCH body by stripping internal-only fields (prefixed with _ or $).
PATCH_BODY=$(jq 'with_entries(select(.key | startswith("_") or startswith("$") | not))
  | del(.requiredResourceAccess[].resourceAccess[]._name)' "$JSON_FILE")

# PATCH the application via Microsoft Graph API.
az rest \
  --method PATCH \
  --url "https://graph.microsoft.com/v1.0/applications/${OBJECT_ID}" \
  --headers "Content-Type=application/json" \
  --body "$PATCH_BODY"

echo ""
echo "Verifying applied metadata..."
az ad app show --id "$APP_ID" \
  --query '{displayName:displayName,description:description,signInAudience:signInAudience,info:info,tags:tags,publicClient:publicClient,isFallbackPublicClient:isFallbackPublicClient}' \
  -o json

echo ""
echo "App registration updated successfully."
echo ""
echo "Preview the consent experience:"
echo "  https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=${APP_ID}&response_type=code&redirect_uri=http%3A%2F%2Flocalhost&scope=Calendars.ReadWrite%20User.Read&prompt=consent"
