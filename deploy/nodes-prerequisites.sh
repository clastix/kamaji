#!/usr/bin/env bash

KUBERNETES_VERSION=$1; shift
HOSTS=("$@")

# Install `containerd` as container runtime.
cat << EOF | tee containerd.conf
overlay
br_netfilter
EOF

cat << EOF | tee 99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t 'sudo apt update && sudo apt install -y containerd'
  ssh ${USER}@${HOST} -t 'sudo systemctl start containerd && sudo systemctl enable containerd'
  scp containerd.conf ${USER}@${HOST}:
  ssh ${USER}@${HOST} -t 'sudo chown -R root:root containerd.conf && sudo mv containerd.conf /etc/modules-load.d/containerd.conf'
  ssh ${USER}@${HOST} -t 'sudo modprobe overlay && sudo modprobe br_netfilter'
  scp 99-kubernetes-cri.conf ${USER}@${HOST}:
  ssh ${USER}@${HOST} -t 'sudo chown -R root:root 99-kubernetes-cri.conf && sudo mv 99-kubernetes-cri.conf /etc/sysctl.d/99-kubernetes-cri.conf'
  ssh ${USER}@${HOST} -t 'sudo sysctl --system'
done

rm -f containerd.conf 99-kubernetes-cri.conf 

# Install `kubectl`, `kubelet`, and `kubeadm` in the desired version.

INSTALL_KUBERNETES="sudo apt install -y kubelet=${KUBERNETES_VERSION}-00 kubeadm=${KUBERNETES_VERSION}-00 kubectl=${KUBERNETES_VERSION}-00 --allow-downgrades --allow-change-held-packages"

for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t 'sudo apt update'
  ssh ${USER}@${HOST} -t 'sudo apt install -y apt-transport-https ca-certificates curl'
  ssh ${USER}@${HOST} -t 'sudo curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg'
  ssh ${USER}@${HOST} -t 'echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list'
  ssh ${USER}@${HOST} -t 'sudo apt update'
  ssh ${USER}@${HOST} -t ${INSTALL_KUBERNETES}
  ssh ${USER}@${HOST} -t 'sudo apt-mark hold kubelet kubeadm kubectl'
done