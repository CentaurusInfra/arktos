load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "interface.go",
        "simple_context.go",
        "simple_test_executor.go",
        "test.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/test",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/api:go_default_library",
        "//perf-tests/clusterloader2/pkg/chaos:go_default_library",
        "//perf-tests/clusterloader2/pkg/config:go_default_library",
        "//perf-tests/clusterloader2/pkg/errors:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/runtimeobjects:go_default_library",
        "//perf-tests/clusterloader2/pkg/state:go_default_library",
        "//perf-tests/clusterloader2/pkg/tuningset:go_default_library",
        "//perf-tests/clusterloader2/pkg/util:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/api/meta:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
