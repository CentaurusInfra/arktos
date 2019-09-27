## Start Workload controller manager from binary (or debug in IDE)
### Set up Kubernetes cluster with kubeadm (v1.15)
### Disable kube controllers implemented in CloudFabric controllers
### systemctl disable kube-controller-manager
### CloudFabric controller manager setup
1. Setup from binary
    a. Copy /etc/kubernetes/admin.conf from kube master into the host that is running CloudFabric controller manager
    b. export KUBECONFIG=<absolution path to admin.conf>
    c. Make sure that the copied admin.conf is accessible from the host
    d. Copy cmd/workload-controller-manager/config/controllerconfig.json to the dir /usr/local/conf
    e. Start the workload-controller-manager by running workload-controller-manager --controllerconfig /usr/local/conf/controllerconfig.json

2. Debug in vscode
    a. Select Debug->Add Configuration ...
    b. Add the following lines into launch.json
           "args": ["--controllerconfig", "/usr/local/conf/controllerconfig.json"]
    c. Start Debugging

3. Add to systemctl
    a. Create a workload-controller-manager.service file in the dir /etc/systemd/system
    b. Add the following line into ExecStart
           --controllerconfig=/usr/local/conf/controllerconfig.json
    c. systemctl daemon-reload
    d. systemctl enable workload-controller-manager
    e. systemctl start workload-controller-manager

