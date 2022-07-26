#!/bin/bash

set -e

# Constants
export DOCKER_IMAGE_NAME="kindest/node"
export DOCKER_NETWORK="kind"

# Variables
export KUBERNETES_VERSION=${1:-v1.23.5}
export KUBECONFIG="${KUBECONFIG:-/tmp/kubeconfig}"

if [ -z $2 ]
then
    MAPPING_PORT=""
else
    MAPPING_PORT="-p ${2}:80"
fi

clear
echo "Welcome to join a new node to the Kind network"

echo -ne "\nChecking right kubeconfig\n"
kubectl cluster-info
echo "Are you pointing to the right tenant control plane? (Type return to continue)"
read

JOIN_CMD="$(kubeadm --kubeconfig=${KUBECONFIG} token create --print-join-command) --ignore-preflight-errors=SystemVerification"
echo "Deploying new node..."
NODE=$(docker run -d --privileged -v /lib/modules:/lib/modules:ro -v /var --net $DOCKER_NETWORK $MAPPING_PORT $DOCKER_IMAGE_NAME:$KUBERNETES_VERSION)
sleep 10
echo "Joining new node..."
docker exec -e JOIN_CMD="$JOIN_CMD" $NODE /bin/bash -c "$JOIN_CMD"

echo "Node has joined! Remember to install the kind-net CNI by issuing the following command:"
echo "  $: kubectl apply -f https://raw.githubusercontent.com/aojea/kindnet/master/install-kindnet.yaml"
