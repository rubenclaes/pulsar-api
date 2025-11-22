#!/bin/bash

# Usage:
#   ./pulsar-sender-multi-recur.sh <kubeconfig_path> <json_file_or_dir> <topic> <client_id> <client_secret> <token_url> [delay_ms]
#
# Examples:
#   Single file:  ./pulsar-sender-multi-recur.sh kubeconfig ./msg.json "persistent://tenant/ns/topic" id secret https://... 500
#   Folder:       ./pulsar-sender-multi-recur.sh kubeconfig ./messages/ "persistent://tenant/ns/topic" id secret https://... 500

KUBECONFIG_PATH="$1"
JSON_PATH="$2"          # file OR directory
TOPIC="$3"
CLIENT_ID="$4"
CLIENT_SECRET="$5"
TOKEN_URL="$6"
DELAY_MS="${7:-500}"    # default 500ms
NAMESPACE="pulsar-acc"
LOCAL_PORT=6650
REMOTE_PORT=6650

if [[ -z "$KUBECONFIG_PATH" || -z "$JSON_PATH" || -z "$TOPIC" || -z "$CLIENT_ID" || -z "$CLIENT_SECRET" || -z "$TOKEN_URL" ]]; then
  echo "Usage: $0 <kubeconfig_path> <json_file_or_dir> <topic> <client_id> <client_secret> <token_url> [delay_ms]"
  exit 1
fi

# Ensure pulsar-client is available locally (same folder structure)
if [[ ! -x "./bin/pulsar-client" ]]; then
  echo "Error: ./bin/pulsar-client not found or not executable. Run from your Pulsar folder."
  exit 1
fi

# Convert delay ms -> seconds (e.g., 500 -> 0.5)
DELAY_SEC=$(awk "BEGIN {printf \"%.3f\", $DELAY_MS/1000}")

# Find broker pod
POD=$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n "$NAMESPACE" get pods -o name | grep proxy | head -n 1 | cut -d'/' -f2)
if [[ -z "$POD" ]]; then
  echo "Broker pod not found."
  exit 1
fi

# Fetch OAuth token
TOKEN_RESPONSE=$(curl -s -X POST "$TOKEN_URL" \
  -d "grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded")

ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"access_token":"[^"]*' | grep -o '[^"]*$')
if [[ -z "$ACCESS_TOKEN" ]]; then
  echo "Failed to fetch access token."
  exit 1
fi

# Start port-forward in background and ensure cleanup
kubectl --kubeconfig "$KUBECONFIG_PATH" -n "$NAMESPACE" port-forward "$POD" $LOCAL_PORT:$REMOTE_PORT >/dev/null 2>&1 &
PF_PID=$!
cleanup() {
  [[ -n "$PF_PID" ]] && kill "$PF_PID" >/dev/null 2>&1
}
trap cleanup EXIT

# Wait for port-forward to be ready
for i in {1..10}; do
  nc -z localhost $LOCAL_PORT && break
  sleep 1
done
if ! nc -z localhost $LOCAL_PORT; then
  echo "Port-forward to broker pod failed."
  exit 1
fi

send_file() {
  local file="$1"
  echo "Sending: $file"
  ./bin/pulsar-client \
    --url pulsar://localhost:$LOCAL_PORT \
    --auth-plugin org.apache.pulsar.client.impl.auth.AuthenticationToken \
    --auth-params "token:$ACCESS_TOKEN" produce \
    -f "$file" \
    -n 1 \
    "$TOPIC"
}

# Build list of files to send
declare -a FILES SENT_FILES FAILED_FILES
if [[ -d "$JSON_PATH" ]]; then
  # Recursief alle *.json files (null-safe, voorspelbaar gesorteerd)
  while IFS= read -r -d '' f; do
    FILES+=("$f")
  done < <(find "$JSON_PATH" -type f -name '*.json' -print0 | sort -z)
  if [[ ${#FILES[@]} -eq 0 ]]; then
    echo "No .json files found in directory (recursively): $JSON_PATH"
    exit 1
  fi
elif [[ -f "$JSON_PATH" ]]; then
  FILES=("$JSON_PATH")
else
  echo "Path not found: $JSON_PATH"
  exit 1
fi

# Send all files with delay between messages
TOTAL=${#FILES[@]}
for idx in "${!FILES[@]}"; do
  file="${FILES[$idx]}"
  if send_file "$file"; then
    SENT_FILES+=("$file")
  else
    FAILED_FILES+=("$file")
  fi

  # Sleep tussen berichten, behalve na de laatste
  if (( idx < TOTAL - 1 )); then
    sleep "$DELAY_SEC"
  fi
done

echo
echo "Done. Attempted: $TOTAL  |  Sent: ${#SENT_FILES[@]}  |  Failed: ${#FAILED_FILES[@]}"
echo

if (( ${#SENT_FILES[@]} > 0 )); then
  echo "Files sent (in order):"
  for i in "${!SENT_FILES[@]}"; do
    printf "  %3d) %s\n" "$((i+1))" "${SENT_FILES[$i]}"
  done
  echo
fi

if (( ${#FAILED_FILES[@]} > 0 )); then
  echo "Files that failed to send:"
  for f in "${FAILED_FILES[@]}"; do
    echo "  - $f"
  done
  echo
fi
