load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["informer.go"],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement/util/informer",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/pkg/measurement/util:go_default_library",
        "//perf-tests/clusterloader2/pkg/util:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//staging/src/k8s.io/client-go/dynamic:go_default_library",
        "//staging/src/k8s.io/client-go/dynamic/dynamicinformer:go_default_library",
        "//staging/src/k8s.io/client-go/tools/cache:go_default_library",
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
