## Start Workload controller manager from binary (or debug in IDE)

### Disable kube controllers implemented in CloudFabric controllers
1. Search for start<Controller> in kube controller manager and comment out
See [disable replicaset controller in kube controller manager](https://github.com/futurewei-cloud/kubernetes/commit/713feb7b9ab6fa52532f908b65d58f819cd56a22) as an example

### Set up Kubernetes cluster
1. Use kubeadm with appropriate version OR
1. Use ./hack/local-up-cluster.sh but disable workload controller manager

### CloudFabric controller manager setup
1. Setup from binary
    1. Copy /etc/kubernetes/admin.conf from kube master into the host that is running CloudFabric controller manager
    1. export KUBECONFIG=<absolution path to admin.conf>
    1. Make sure that the copied admin.conf is accessible from the host
    1. Copy cmd/workload-controller-manager/config/controllerconfig.json to the dir /usr/local/conf
    1. Start the workload-controller-manager by running workload-controller-manager --controllerconfig /usr/local/conf/controllerconfig.json

1. Debug in vscode OR
    1. Select Debug->Add Configuration ...
    1. Add the following lines into launch.json
           "args": ["--controllerconfig", "/usr/local/conf/controllerconfig.json"]
    1. Start Debugging

1. Debug in Goland

## Use field selector in Kubectl
Example: pod has hashkey as 0
```
$ kubectl get pod --all-namespaces --field-selector metadata.hashkey=0
NAMESPACE     NAME                       HASHKEY   READY   STATUS    RESTARTS   AGE
kube-system   kube-dns-59bc784c6-spjkh   0         3/3     Running   0          31s
```
#### Get pod with hashkey <= -1
```
$ kubectl get pod --all-namespaces --field-selector metadata.hashkey=lte:-1
  No resources found.
```
#### Get pod with hashkey <= 0
```
$ kubectl get pod --all-namespaces --field-selector metadata.hashkey=lte:0
  NAMESPACE     NAME                       HASHKEY   READY   STATUS    RESTARTS   AGE
  kube-system   kube-dns-59bc784c6-spjkh   0         3/3     Running   0          3m24s
```

#### Get pod with -10 < hashkey <= 10 
```
$ kubectl get pod --all-namespaces --field-selector metadata.hashkey=lte:10,metadata.hashkey=gt:-10
  NAMESPACE     NAME                       HASHKEY   READY   STATUS    RESTARTS   AGE
  kube-system   kube-dns-59bc784c6-spjkh   0         3/3     Running   0          4m
```

#### Get pod with -10 < hashKey <= -1
```  
$ kubectl get pod --all-namespaces --field-selector metadata.hashkey=lte:-1,metadata.hashkey=gt:-10
  No resources found.
```

#### Get pod with hashKey > 10000  or <= 100
```  
$ kubectl get pod --all-namespaces --field-selector metadata.hashkey=gt:10000\;metadata.hashkey=lte:100
  No resources found.
```

#### Get pod with owner references replicaset hashkey as 0
```  
$ kubectl get pod --all-namespaces --field-selector metadata.ownerReferences.hashkey.ReplicaSet=0
 NAMESPACE     NAME                       HASHKEY   READY   STATUS              RESTARTS   AGE
 default       busybox-7cc9f9c486-8758n   0         1/1     Running             0          3m
```

#### Get pod with owner references replicaset hashkey <=-1
```  
$ kubectl get pod --all-namespaces --field-selector metadata.ownerReferences.hashkey.ReplicaSet=lte:-1
  No resources found.
```

#### Get pod with owner references replicaset hashkey > -1
```  
$ kubectl get pod --all-namespaces --field-selector metadata.ownerReferences.hashkey.ReplicaSet=gt:-1
 NAMESPACE     NAME                       HASHKEY   READY   STATUS              RESTARTS   AGE
 default       busybox-7cc9f9c486-8758n   0         1/1     Running             0          3m
```

#### Get pod with -10 <= owner references replicaset  hashkey < 10 
```  
$ kubectl get pod --all-namespaces --field-selector metadata.ownerReferences.hashkey.ReplicaSet=gte:-10,metadata.ownerReferences.hashkey.ReplicaSet=lt:10
 NAMESPACE     NAME                       HASHKEY   READY   STATUS              RESTARTS   AGE
 default       busybox-7cc9f9c486-8758n   0         1/1     Running             0          3m
```

#### Get pod with -10 < owner references replicaset  hashkey <= -1
```
$ kubectl get pod --all-namespaces --field-selector metadata.ownerReferences.hashkey.ReplicaSet=gt:-10,metadata.ownerReferences.hashkey.ReplicaSet=lte:-1
 No resources found.
```