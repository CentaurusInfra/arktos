load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["revoke.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/revoke",
    importpath = "github.com/cloudflare/cfssl/revoke",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/golang.org/x/crypto/ocsp:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["revoke_test.go"],
    embed = [":go_default_library"],
    tags = ["manual"],
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
