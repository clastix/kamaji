ARG KUBERNETES_VERSION=v1.23.4
FROM kindest/node:$KUBERNETES_VERSION

COPY ./cni-kindnet-config.json /etc/cni/net.d/10-kindnet.conflist
