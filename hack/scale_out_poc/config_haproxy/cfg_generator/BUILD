load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "cfg_generator.go",
        "util.go",
    ],
    importpath = "k8s.io/kubernetes/hack/scale_out_poc/config_haproxy/cfg_generator",
    visibility = ["//visibility:private"],
    deps = [
        "//staging/src/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_binary(
    name = "cfg_generator",
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
