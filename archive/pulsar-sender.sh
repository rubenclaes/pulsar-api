#!/bin/bash

KUBECONFIG_PATH="$1"
JSON_FILE="$2"
TOPIC="$3"
CLIENT_ID="$4"
CLIENT_SECRET="$5"
TOKEN_URL="$6"
NAMESPACE="pulsar-acc"
LOCAL_PORT=6650
REMOTE_PORT=6650

if [[ -z "$KUBECONFIG_PATH" || -z "$JSON_FILE" || -z "$TOPIC" || -z "$CLIENT_ID" || -z "$CLIENT_SECRET" || -z "$TOKEN_URL" ]]; then
  echo "Usage: $0 <kubeconfig_path> <json_file> <topic> <client_id> <client_secret> <token_url>"
  exit 1
fi

POD=$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n $NAMESPACE get pods -o name | grep proxy | head -n 1 | cut -d'/' -f2)
if [[ -z "$POD" ]]; then
  echo "Broker pod not found."
  exit 1
fi

TOKEN_RESPONSE=$(curl -s -X POST $TOKEN_URL \
  -d "grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded")

ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"access_token":"[^"]*' | grep -o '[^"]*$')
if [[ -z "$ACCESS_TOKEN" ]]; then
  echo "Failed to fetch access token."
  exit 1
fi

# Start port-forward in background
kubectl --kubeconfig "$KUBECONFIG_PATH" -n $NAMESPACE port-forward "$POD" $LOCAL_PORT:$REMOTE_PORT >/dev/null 2>&1 &
PF_PID=$!

# Wait for port-forward to be ready
for i in {1..10}; do
  nc -z localhost $LOCAL_PORT && break
  sleep 1
done

if ! nc -z localhost $LOCAL_PORT; then
  echo "Port-forward to broker pod failed."
  kill $PF_PID
  exit 1
fi

# Use local pulsar-client
./bin/pulsar-client \
  --url pulsar://localhost:$LOCAL_PORT \
  --auth-plugin org.apache.pulsar.client.impl.auth.AuthenticationToken \
  --auth-params "token:$ACCESS_TOKEN" produce \
  -f "$JSON_FILE"  \
  -r 1 \
  "$TOPIC"

# Clean up port-forward
kill $PF_PID
