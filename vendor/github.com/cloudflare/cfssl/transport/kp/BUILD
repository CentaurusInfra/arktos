load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["key_provider.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/transport/kp",
    importpath = "github.com/cloudflare/cfssl/transport/kp",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/csr:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/core:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["key_provider_test.go"],
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/csr:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/core:go_default_library",
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
