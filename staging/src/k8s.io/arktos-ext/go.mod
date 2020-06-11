module k8s.io/arktos-ext

go 1.12

replace (
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/client-go => ../client-go
)

require (
	github.com/grafov/bcast v0.0.0-20190217190352-1447f067e08d // indirect
	github.com/kr/pretty v0.2.0 // indirect
	gopkg.in/fatih/set.v0 v0.2.1 // indirect
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0-00010101000000-000000000000
	k8s.io/klog v1.0.0
)
