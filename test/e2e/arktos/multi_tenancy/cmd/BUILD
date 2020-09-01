load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["testrunner.go"],
    importpath = "k8s.io/kubernetes/test/e2e/arktos/multi_tenancy/cmd",
    visibility = ["//visibility:private"],
    deps = [
        "//test/e2e/arktos/multi_tenancy/cmd/framework:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_binary(
    name = "cmd",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
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
        "//test/e2e/arktos/multi_tenancy/cmd/framework:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
