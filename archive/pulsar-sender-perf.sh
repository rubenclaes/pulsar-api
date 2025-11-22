#!/bin/bash

# Usage:
#   ./send-perf.sh <kubeconfig_path> <json_file> <topic> <client_id> <client_secret> <token_url> [rate] [num_messages]
#
# Examples:
#   Every 500ms (rate=2), send 100 messages:
#     ./send-perf.sh /home/you/.kube/config ./message.json "persistent://tenant/ns/topic" id secret https://... 2 100
#   1000 msg/s, 50k messages:
#     ./send-perf.sh /home/you/.kube/config ./message.json "persistent://tenant/ns/topic" id secret https://... 1000 50000

KUBECONFIG_PATH="$1"
JSON_FILE="$2"
TOPIC="$3"
CLIENT_ID="$4"
CLIENT_SECRET="$5"
TOKEN_URL="$6"
RATE="${7:-2}"            # msgs/sec (default ~500ms/message)
NUM_MESSAGES="${8:-100}"  # total messages (0 = run forever)

NAMESPACE="pulsar-acc"
LOCAL_PORT=6650
REMOTE_PORT=6650

if [[ -z "$KUBECONFIG_PATH" || -z "$JSON_FILE" || -z "$TOPIC" || -z "$CLIENT_ID" || -z "$CLIENT_SECRET" || -z "$TOKEN_URL" ]]; then
  echo "Usage: $0 <kubeconfig_path> <json_file> <topic> <client_id> <client_secret> <token_url> [rate] [num_messages]"
  exit 1
fi

# Ensure required binaries
if [[ ! -x "./bin/pulsar-perf" ]]; then
  echo "Error: ./bin/pulsar-perf not found or not executable. Run from your Pulsar folder (apache-pulsar-4.0.6)."
  exit 1
fi

if [[ ! -f "$JSON_FILE" ]]; then
  echo "JSON file not found: $JSON_FILE"
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

# Prepare a one-line payload file for pulsar-perf
TMP_PAYLOAD="$(mktemp)"
cleanup() {
  [[ -n "$PF_PID" ]] && kill "$PF_PID" >/dev/null 2>&1
  [[ -f "$TMP_PAYLOAD" ]] && rm -f "$TMP_PAYLOAD"
}
trap cleanup EXIT

if command -v jq >/dev/null 2>&1; then
  if ! jq -c . "$JSON_FILE" > "$TMP_PAYLOAD"; then
    echo "Warning: jq failed to compact JSON. Using raw file as-is."
    cat "$JSON_FILE" > "$TMP_PAYLOAD"
  fi
else
  echo "Note: jq not found; ensure $JSON_FILE is a single-line JSON. Using as-is."
  cat "$JSON_FILE" > "$TMP_PAYLOAD"
fi

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
  --rate "$RATE" \
  --num-messages "$NUM_MESSAGES"

STATUS=$?
if [[ $STATUS -eq 0 ]]; then
  echo "pulsar-perf completed successfully."
else
  echo "pulsar-perf exited with status $STATUS."
fi
