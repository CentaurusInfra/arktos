load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "cfg_generator.go",
        "util.go",
    ],
    importpath = "k8s.io/kubernetes/cmd/haproxy-cfg-generator",
    visibility = ["//visibility:private"],
    deps = ["//vendor/k8s.io/klog:go_default_library"],
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

go_test(
    name = "go_default_test",
    srcs = ["cfg_generator_test.go"],
    data = glob(["data/**"]),
    embed = [":go_default_library"],
)
