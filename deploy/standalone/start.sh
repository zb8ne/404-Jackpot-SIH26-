#!/bin/sh
# Boots Anvil and the Go server in one container, talking over loopback only.
# This exists for hosts where a second networked service for the chain is
# more trouble than it is worth: there is no cross-service network path here
# for anything to go wrong on.
set -e

DATA_DIR="${DATA_DIR:-/data}"
STATE_FILE="$DATA_DIR/anvil-state.json"
DEPLOYMENT_FILE="$DATA_DIR/deployment.txt"
mkdir -p "$DATA_DIR"

echo "starting anvil (state: $STATE_FILE)"
anvil --host 127.0.0.1 --port 8545 --silent --state "$STATE_FILE" &
ANVIL_PID=$!

echo "waiting for anvil..."
for i in $(seq 1 30); do
  cast block-number --rpc-url http://127.0.0.1:8545 >/dev/null 2>&1 && break
  sleep 1
done
cast block-number --rpc-url http://127.0.0.1:8545 >/dev/null 2>&1 || {
  echo "anvil never came up"; exit 1;
}
echo "anvil up"

if [ -f "$DEPLOYMENT_FILE" ]; then
  echo "registry already deployed at $(cat "$DEPLOYMENT_FILE") (state persisted on the volume)"
else
  echo "deploying the registry (first boot on this volume)..."
  cd /contracts
  DEPLOYER_KEY="${DEPLOYER_KEY:-0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80}" \
    forge script script/Deploy.s.sol:Deploy --rpc-url http://127.0.0.1:8545 --broadcast
  cp deployment.txt "$DEPLOYMENT_FILE"
  cd /
  echo "registry deployed at $(cat "$DEPLOYMENT_FILE")"
fi

export RPC_URL="http://127.0.0.1:8545"
export CONTRACT_ADDRESS="$(cat "$DEPLOYMENT_FILE")"
export DB_PATH="$DATA_DIR/credentials.db"

# If anvil dies, the whole container should die and restart cleanly rather
# than serve a backend with no chain underneath it.
trap 'kill $ANVIL_PID 2>/dev/null' EXIT

exec /usr/local/bin/server
