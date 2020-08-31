load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["kubelet.go"],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement/util/kubelet",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/pkg/measurement/util:go_default_library",
        "//pkg/kubelet/apis/stats/v1alpha1:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
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
