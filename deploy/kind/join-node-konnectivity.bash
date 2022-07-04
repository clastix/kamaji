#!/bin/bash

set -e

# Constants
export DOCKER_IMAGE_NAME="clastix/kamaji-kind-worker"

# Variables
export KUBERNETES_VERSION=${1:-latest}
export KUBECONFIG="${KUBECONFIG:-/tmp/kubeconfig}"

if [ -z $2 ]
then
    MAPPING_PORT=""
else
    MAPPING_PORT="-p ${2}:80"
fi

export KONNECTIVITY_PROXY_HOST=${3:-konnectiviy.local}

clear
echo "Welcome to join a new node through Konnectivity"

echo -ne "\nChecking right kubeconfig\n"
kubectl cluster-info
echo "Are you pointing to the right tenant control plane? (Type return to continue)"
read

JOIN_CMD="$(kubeadm --kubeconfig=${KUBECONFIG} token create --print-join-command) --ignore-preflight-errors=SystemVerification"
echo "Deploying new node..."
KIND_IP=$(docker inspect kamaji-control-plane --format='{{.NetworkSettings.Networks.kind.IPAddress}}')
NODE=$(docker run -d --add-host $KONNECTIVITY_PROXY_HOST:$KIND_IP --privileged -v /lib/modules:/lib/modules:ro -v /var --net host $MAPPING_PORT $DOCKER_IMAGE_NAME:$KUBERNETES_VERSION)
sleep 10
echo "Joining new node..."
docker exec -e JOIN_CMD="$JOIN_CMD" $NODE /bin/bash -c "$JOIN_CMD"
