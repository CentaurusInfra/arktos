load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "framework.go",
        "multiclient.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/framework",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/pkg/config:go_default_library",
        "//perf-tests/clusterloader2/pkg/errors:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework/client:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework/config:go_default_library",
        "//perf-tests/clusterloader2/pkg/util:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/api/meta:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/client-go/discovery/cached:go_default_library",
        "//staging/src/k8s.io/client-go/dynamic:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//staging/src/k8s.io/client-go/plugin/pkg/client/auth:go_default_library",
        "//staging/src/k8s.io/client-go/restmapper:go_default_library",
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
        "//perf-tests/clusterloader2/pkg/framework/client:all-srcs",
        "//perf-tests/clusterloader2/pkg/framework/config:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
