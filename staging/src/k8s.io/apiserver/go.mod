// This is a generated file. Do not edit directly.

module k8s.io/apiserver

go 1.12

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/NYTimes/gziphandler v0.0.0-20170623195520-56545f4a5d46 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/coreos/bbolt v1.3.1-coreos.6 // indirect
	github.com/coreos/etcd v3.3.15+incompatible
	github.com/coreos/go-oidc v0.0.0-20180117170138-065b426bd416
	github.com/coreos/go-semver v0.0.0-20180108230905-e214231b295a // indirect
	github.com/coreos/go-systemd v0.0.0-20180511133405-39ca1b05acc7
	github.com/coreos/pkg v0.0.0-20180108230652-97fdf19511ea
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/evanphx/json-patch v4.2.0+incompatible
	github.com/ghodss/yaml v0.0.0-20180820084758-c7ce16629ff4 // indirect
	github.com/go-openapi/jsonpointer v0.19.0 // indirect
	github.com/go-openapi/jsonreference v0.19.0 // indirect
	github.com/go-openapi/spec v0.19.2
	github.com/go-openapi/swag v0.17.2 // indirect
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/google/gofuzz v1.0.0
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/grafov/bcast v0.0.0-20190217190352-1447f067e08d
	github.com/grpc-ecosystem/go-grpc-middleware v0.0.0-20190222133341-cfaf5686ec79 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v0.0.0-20170330212424-2500245aa611
	github.com/grpc-ecosystem/grpc-gateway v1.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.1
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/pborman/uuid v1.2.0
	github.com/pkg/errors v0.8.0 // indirect
	github.com/pquerna/cachecontrol v0.0.0-20171018203845-0dec1b30a021 // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20180712105110-5c3871d89910
	github.com/sirupsen/logrus v1.2.0 // indirect
	github.com/soheilhy/cmux v0.1.3 // indirect
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	github.com/tmc/grpc-websocket-proxy v0.0.0-20170815181823-89b8d40f7ca8 // indirect
	github.com/xiang90/probing v0.0.0-20160813154853-07dd2e8dfe18 // indirect
	go.uber.org/atomic v0.0.0-20181018215023-8dc6146f7569 // indirect
	go.uber.org/multierr v0.0.0-20180122172545-ddea229ff1df // indirect
	go.uber.org/zap v0.0.0-20180814183419-67bc79d13d15 // indirect
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	google.golang.org/genproto v0.0.0-20170731182057-09f6ed296fc6 // indirect
	google.golang.org/grpc v1.23.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/square/go-jose.v2 v2.2.2
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0 // indirect
	gopkg.in/yaml.v2 v2.2.4
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v0.4.0
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf
	k8s.io/utils v0.0.0-20190801114015-581e00157fb1
	sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
	sigs.k8s.io/yaml v1.1.0
)

replace (
	github.com/BurntSushi/toml => github.com/BurntSushi/toml v0.3.0
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.0
	github.com/coreos/etcd => github.com/coreos/etcd v3.3.13+incompatible
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go v0.0.0-20160705203006-01aeca54ebda
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v0.0.0-20170410110728-ff4f55a20633
	github.com/evanphx/json-patch => github.com/evanphx/json-patch v0.0.0-20190203023257-5858425f7550
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.17.2
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
	k8s.io/apimachinery => ../apimachinery
	k8s.io/apiserver => ../apiserver
	k8s.io/client-go => ../client-go
	k8s.io/component-base => ../component-base
	k8s.io/klog => k8s.io/klog v0.3.1
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30
	k8s.io/utils => k8s.io/utils v0.0.0-20190221042446-c2654d5206da
)
