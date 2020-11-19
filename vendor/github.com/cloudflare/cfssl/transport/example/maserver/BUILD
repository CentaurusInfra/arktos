load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["server.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/transport/example/maserver",
    importpath = "github.com/cloudflare/cfssl/transport/example/maserver",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/core:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/example/exlib:go_default_library",
    ],
)

go_binary(
    name = "maserver",
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
