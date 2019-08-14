## Start Workload controll manager from binary (or debug in IDE)
### Set up Kubernetes cluster with kubeadm (v1.15)
### Disable kube controllers implemented in CloudFabric controllers
### CloudFabric controller manager setup
1. Setup Kube certificates
    1. Copy /etc/kubernetes/admin.conf from kube master into the host that is running CloudFabric controller manager
    1. export KUBECONFIG=<absolution path to admin.conf>
    1. Make sure server in admin.conf is accessible from the host
1. Setup CloudFabric controller manager configuration in ETCD
    1. $kubectl apply -f crd.yaml
    1. $kubectl apply -f controllermanager.yaml


