load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "container_resource_gatherer.go",
        "resource_gather_worker.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement/util/gatherers",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/pkg/measurement/util:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/kubelet:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/kubemark:go_default_library",
        "//pkg/util/system:go_default_library",
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
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
