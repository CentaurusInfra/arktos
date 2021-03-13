// This is a generated file. Do not edit directly.

module k8s.io/apiextensions-apiserver

go 1.13

require (
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/go-openapi/errors v0.19.2
	github.com/go-openapi/spec v0.19.3
	github.com/go-openapi/strfmt v0.19.3
	github.com/go-openapi/validate v0.19.5
	github.com/gogo/protobuf v1.3.1
	github.com/google/go-cmp v0.3.0
	github.com/google/gofuzz v1.1.0
	github.com/googleapis/gnostic v0.1.0
	github.com/pborman/uuid v1.2.0
	github.com/prometheus/client_golang v1.1.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200224211402-c65a9e2dd1fd
	google.golang.org/grpc v1.26.0
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/code-generator v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89
	sigs.k8s.io/yaml v1.2.0
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.34.0

	github.com/coreos/go-oidc => github.com/coreos/go-oidc v0.0.0-20180117170138-065b426bd416
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go v0.0.0-20160705203006-01aeca54ebda
	github.com/docker/go-units => github.com/docker/go-units v0.3.3
	github.com/golang/mock => github.com/golang/mock v0.0.0-20160127222235-bd3c8e81be01
	github.com/google/gofuzz => github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d
	github.com/grpc-ecosystem/go-grpc-middleware => github.com/grpc-ecosystem/go-grpc-middleware v0.0.0-20190222133341-cfaf5686ec79
	github.com/grpc-ecosystem/go-grpc-prometheus => github.com/grpc-ecosystem/go-grpc-prometheus v0.0.0-20170330212424-2500245aa611
	github.com/grpc-ecosystem/grpc-gateway => github.com/grpc-ecosystem/grpc-gateway v1.3.0
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.5.0
	github.com/jonboulle/clockwork => github.com/jonboulle/clockwork v0.0.0-20141017032234-72f9bd7c4e0c
	github.com/konsorten/go-windows-terminal-sequences => github.com/konsorten/go-windows-terminal-sequences v1.0.1
	github.com/mailru/easyjson => github.com/mailru/easyjson v0.0.0-20190626092158-b2ccc519800e
	github.com/mattn/go-isatty => github.com/mattn/go-isatty v0.0.9
	github.com/munnerz/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/common => github.com/prometheus/common v0.0.0-20181126121408-4724e9255275
	github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
	github.com/soheilhy/cmux => github.com/soheilhy/cmux v0.1.3
	github.com/xiang90/probing => github.com/xiang90/probing v0.0.0-20160813154853-07dd2e8dfe18
	go.uber.org/atomic => go.uber.org/atomic v0.0.0-20181018215023-8dc6146f7569
	go.uber.org/multierr => go.uber.org/multierr v0.0.0-20180122172545-ddea229ff1df
	go.uber.org/zap => go.uber.org/zap v0.0.0-20180814183419-67bc79d13d15
	golang.org/x/sync => golang.org/x/sync v0.0.0-20181108010431-42b317875d0f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4
	golang.org/x/text => golang.org/x/text v0.3.1-0.20181227161524-e6919f6577db
	gonum.org/v1/gonum => gonum.org/v1/gonum v0.0.0-20190331200053-3d26580ed485
	gopkg.in/check.v1 => gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.2.2
	k8s.io/api => ../api
	k8s.io/apiextensions-apiserver => ../apiextensions-apiserver
	k8s.io/apimachinery => ../apimachinery
	k8s.io/apiserver => ../apiserver
	k8s.io/client-go => ../client-go
	k8s.io/code-generator => ../code-generator
	k8s.io/component-base => ../component-base
	k8s.io/utils => k8s.io/utils v0.0.0-20190221042446-c2654d5206da
	sigs.k8s.io/yaml => sigs.k8s.io/yaml v1.1.0
)
