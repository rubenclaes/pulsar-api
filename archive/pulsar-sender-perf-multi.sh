#!/bin/bash

# Usage:
#   ./send-perf-dir.sh <kubeconfig_path> <json_dir> <topic> <client_id> <client_secret> <token_url> [rate] [num_messages]
#
# Example:
#   ./send-perf-dir.sh /home/you/.kube/config ./messages \
#     "persistent://tenant/pulsar-acc/orders" id secret https://... 200 0
#
# Notes:
#   - Each *.json file in <json_dir> becomes one payload line (randomly chosen per message by pulsar-perf).
#   - Requires ./bin/pulsar-perf (run from apache-pulsar-4.0.6 directory).

KUBECONFIG_PATH="$1"
JSON_DIR="$2"
TOPIC="$3"
CLIENT_ID="$4"
CLIENT_SECRET="$5"
TOKEN_URL="$6"
RATE="${7:-2}"            # msgs/sec
NUM_MESSAGES="${8:-100}"  # 0 = run forever

NAMESPACE="pulsar-acc"
LOCAL_PORT=6650
REMOTE_PORT=6650

if [[ -z "$KUBECONFIG_PATH" || -z "$JSON_DIR" || -z "$TOPIC" || -z "$CLIENT_ID" || -z "$CLIENT_SECRET" || -z "$TOKEN_URL" ]]; then
  echo "Usage: $0 <kubeconfig_path> <json_dir> <topic> <client_id> <client_secret> <token_url> [rate] [num_messages]"
  exit 1
fi

# Binaries
if [[ ! -x "./bin/pulsar-perf" ]]; then
  echo "Error: ./bin/pulsar-perf not found or not executable. Run from your Pulsar folder (apache-pulsar-4.0.6)."
  exit 1
fi

# Input dir checks
if [[ ! -d "$JSON_DIR" ]]; then
  echo "Error: JSON directory not found: $JSON_DIR"
  exit 1
fi

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

# Temp payload list (one payload per line)
TMP_PAYLOAD="$(mktemp)"
cleanup() {
  [[ -n "$PF_PID" ]] && kill "$PF_PID" >/dev/null 2>&1
  [[ -f "$TMP_PAYLOAD" ]] && rm -f "$TMP_PAYLOAD"
}
trap cleanup EXIT

# Build payload file: compact each JSON into a single line
COUNT=0
shopt -s nullglob
mapfile -t FILES < <(find "$JSON_DIR" -maxdepth 1 -type f -name '*.json' | sort)
if (( ${#FILES[@]} == 0 )); then
  echo "No .json files found in directory: $JSON_DIR"
  exit 1
fi

if command -v jq >/dev/null 2>&1; then
  for f in "${FILES[@]}"; do
    if jq -c . "$f" >> "$TMP_PAYLOAD"; then
      ((COUNT++))
    else
      echo "Warning: Skipping invalid JSON: $f"
    fi
  done
else
  echo "Note: jq not found; using raw files compacted by stripping newlines."
  for f in "${FILES[@]}"; do
    tr -d '\n' < "$f" >> "$TMP_PAYLOAD"
    echo >> "$TMP_PAYLOAD"
    ((COUNT++))
  done
fi

if (( COUNT == 0 )); then
  echo "No valid payloads built from: $JSON_DIR"
  exit 1
fi

echo "Built payload list with $COUNT entries from: $JSON_DIR"

# Start port-forward in background
kubectl --kubeconfig "$KUBECONFIG_PATH" -n "$NAMESPACE" port-forward "$POD" $LOCAL_PORT:$REMOTE_PORT >/dev/null 2>&1 &
PF_PID=$!

# Wait for port-forward to be ready
for i in {1..10}; do
  nc -z localhost $LOCAL_PORT && break
  sleep 1
done

if ! nc -z localhost $LOCAL_PORT; then
  echo "Port-forward to broker pod failed."
  exit 1
fi

echo "Starting pulsar-perf: rate=${RATE} msg/s, num-messages=${NUM_MESSAGES}, topic=${TOPIC}"
./bin/pulsar-perf produce "$TOPIC" \
  --service-url "pulsar://localhost:$LOCAL_PORT" \
  --auth-plugin org.apache.pulsar.client.impl.auth.AuthenticationToken \
  --auth-params "token:$ACCESS_TOKEN" \
  --payload-file "$TMP_PAYLOAD" \
  --payload-delimiter $'\n' \
  --rate "$RATE" \
  --num-messages "$NUM_MESSAGES"

STATUS=$?
if [[ $STATUS -eq 0 ]]; then
  echo "pulsar-perf completed successfully."
else
  echo "pulsar-perf exited with status $STATUS."
fi