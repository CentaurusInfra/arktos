load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["coff.go"],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/akavel/rsrc/coff",
    importpath = "github.com/akavel/rsrc/coff",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/cloudflare/cfssl/vendor/github.com/akavel/rsrc/binutil:go_default_library"],
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
