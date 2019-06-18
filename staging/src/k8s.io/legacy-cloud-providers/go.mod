// This is a generated file. Do not edit directly.

module k8s.io/legacy-cloud-providers

go 1.12

require (
	cloud.google.com/go v0.34.0
	github.com/Azure/azure-sdk-for-go v21.4.0+incompatible
	github.com/Azure/go-autorest v11.1.2+incompatible
	github.com/GoogleCloudPlatform/k8s-cloud-provider v0.0.0-20181220005116-f8e995905100
	github.com/aws/aws-sdk-go v1.16.26
	github.com/dnaeon/go-vcr v1.0.1 // indirect
	github.com/marstr/guid v0.0.0-20170427235115-8bdf7d1a087c // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/rubiojr/go-vhd v0.0.0-20160810183302-0bfd3b39853c
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/stretchr/testify v1.2.2
	github.com/vmware/govmomi v0.20.1
	golang.org/x/crypto v0.0.0-20181025213731-e84da0312774
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a
	google.golang.org/api v0.0.0-20181220000619-583d854617af
	gopkg.in/gcfg.v1 v1.2.0
	gopkg.in/warnings.v0 v0.1.1 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/cloud-provider v0.0.0
	k8s.io/csi-translation-lib v0.0.0
	k8s.io/klog v0.3.1
	k8s.io/utils v0.0.0-20190221042446-c2654d5206da
	sigs.k8s.io/yaml v1.1.0
)

replace (
	golang.org/x/sync => golang.org/x/sync v0.0.0-20181108010431-42b317875d0f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190209173611-3b5209105503
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190313210603-aa82965741a9
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/client-go => ../client-go
	k8s.io/cloud-provider => ../cloud-provider
	k8s.io/csi-translation-lib => ../csi-translation-lib
	k8s.io/legacy-cloud-providers => ../legacy-cloud-providers
)
