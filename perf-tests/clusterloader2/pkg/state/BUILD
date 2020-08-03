load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "namespaces_state.go",
        "resource_state.go",
        "state.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/state",
    visibility = ["//visibility:public"],
    deps = ["//perf-tests/clusterloader2/api:go_default_library"],
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
