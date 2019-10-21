// This is a generated file. Do not edit directly.

module k8s.io/apiextensions-apiserver

go 1.12

require (
	github.com/coreos/etcd v3.3.15+incompatible
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/globalsign/mgo v0.0.0-20181015135952-eeefdecb41b8 // indirect
	github.com/go-openapi/analysis v0.17.2 // indirect
	github.com/go-openapi/errors v0.17.2 // indirect
	github.com/go-openapi/loads v0.19.2 // indirect
	github.com/go-openapi/runtime v0.17.2 // indirect
	github.com/go-openapi/spec v0.19.2
	github.com/go-openapi/strfmt v0.19.0
	github.com/go-openapi/validate v0.19.2
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d
	github.com/google/go-cmp v0.3.0
	github.com/google/gofuzz v1.0.0
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/pborman/uuid v1.2.0
	github.com/prometheus/client_golang v0.9.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/code-generator v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v0.4.0
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf
	k8s.io/utils v0.0.0-20190801114015-581e00157fb1
	sigs.k8s.io/yaml v1.1.0
)

replace (
	github.com/BurntSushi/toml => github.com/BurntSushi/toml v0.3.0
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.0
	github.com/coreos/etcd => github.com/coreos/etcd v3.3.13+incompatible
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go v0.0.0-20160705203006-01aeca54ebda
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v0.0.0-20170410110728-ff4f55a20633
	github.com/evanphx/json-patch => github.com/evanphx/json-patch v0.0.0-20190203023257-5858425f7550
	github.com/go-openapi/loads => github.com/go-openapi/loads v0.17.2
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.17.2
	github.com/go-openapi/strfmt => github.com/go-openapi/strfmt v0.17.0
	github.com/go-openapi/validate => github.com/go-openapi/validate v0.18.0
	github.com/gogo/protobuf => github.com/gogo/protobuf v0.0.0-20171007142547-342cbe0a0415
	github.com/golang/protobuf => github.com/golang/protobuf v1.2.0
	github.com/google/gofuzz => github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf
	github.com/google/uuid => github.com/google/uuid v1.0.0
	github.com/gorilla/websocket => github.com/gorilla/websocket v0.0.0-20170926233335-4201258b820c
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.5.0
	github.com/jonboulle/clockwork => github.com/jonboulle/clockwork v0.0.0-20141017032234-72f9bd7c4e0c
	github.com/json-iterator/go => github.com/json-iterator/go v0.0.0-20180701071628-ab8a2e0c74be
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega => github.com/onsi/gomega v0.0.0-20190113212917-5533ce8a0da3
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.0-20180319062004-c439c4fa0937
	github.com/spf13/pflag => github.com/spf13/pflag v1.0.1
	github.com/stretchr/testify => github.com/stretchr/testify v1.2.2
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20181025213731-e84da0312774
	golang.org/x/net => golang.org/x/net v0.0.0-20190206173232-65e2d4e15006
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a
	golang.org/x/sync => golang.org/x/sync v0.0.0-20181108010431-42b317875d0f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190209173611-3b5209105503
	golang.org/x/time => golang.org/x/time v0.0.0-20161028155119-f51c12702a4d
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190313210603-aa82965741a9
	google.golang.org/grpc => google.golang.org/grpc v1.13.0
	gopkg.in/check.v1 => gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405
	gopkg.in/natefinch/lumberjack.v2 => gopkg.in/natefinch/lumberjack.v2 v2.0.0-20150622162204-20b71e5b60d7
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.0.0-20180411045311-89060dee6a84
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.1
	k8s.io/api => ../api
	k8s.io/apiextensions-apiserver => ../apiextensions-apiserver
	k8s.io/apimachinery => ../apimachinery
	k8s.io/apiserver => ../apiserver
	k8s.io/client-go => ../client-go
	k8s.io/code-generator => ../code-generator
	k8s.io/component-base => ../component-base
	k8s.io/gengo => k8s.io/gengo v0.0.0-20190116091435-f8a0810f38af
	k8s.io/klog => k8s.io/klog v0.3.1
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30
	k8s.io/utils => k8s.io/utils v0.0.0-20190221042446-c2654d5206da
)
