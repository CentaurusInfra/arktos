load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["report.go"],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/report",
    importpath = "github.com/cloudflare/redoctober/report",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/getsentry/raven-go:go_default_library",
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
