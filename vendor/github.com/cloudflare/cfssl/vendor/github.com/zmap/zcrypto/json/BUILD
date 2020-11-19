load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "dhe.go",
        "ecdhe.go",
        "names.go",
        "rsa.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/json",
    importpath = "github.com/zmap/zcrypto/json",
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
