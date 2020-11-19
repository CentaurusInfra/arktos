load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["example_whitelist.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/whitelist/example",
    importpath = "github.com/cloudflare/cfssl/whitelist/example",
    visibility = ["//visibility:private"],
    deps = ["//vendor/github.com/cloudflare/cfssl/whitelist:go_default_library"],
)

go_binary(
    name = "example",
    embed = [":go_default_library"],
    tags = ["manual"],
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
