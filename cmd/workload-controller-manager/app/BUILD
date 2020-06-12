load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "controllermanager.go",
        "helper.go",
        "serve.go",
    ],
    importpath = "k8s.io/kubernetes/cmd/workload-controller-manager/app",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/workload-controller-manager/app/config:go_default_library",
        "//pkg/api/legacyscheme:go_default_library",
        "//pkg/cloudfabric-controller:go_default_library",
        "//pkg/cloudfabric-controller/controllerframework:go_default_library",
        "//pkg/cloudfabric-controller/deployment:go_default_library",
        "//pkg/cloudfabric-controller/replicaset:go_default_library",
        "//pkg/util/configz:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/endpoints/filters:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/endpoints/request:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/server:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/server/filters:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/server/healthz:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/server/mux:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/server/routes:go_default_library",
        "//staging/src/k8s.io/client-go/datapartition:go_default_library",
        "//staging/src/k8s.io/client-go/discovery/cached:go_default_library",
        "//staging/src/k8s.io/client-go/informers:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//staging/src/k8s.io/client-go/restmapper:go_default_library",
        "//staging/src/k8s.io/component-base/config:go_default_library",
        "//vendor/github.com/grafov/bcast:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//cmd/workload-controller-manager/app/config:all-srcs",
        "//cmd/workload-controller-manager/app/options:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
