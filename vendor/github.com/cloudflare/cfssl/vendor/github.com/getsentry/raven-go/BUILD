load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "client.go",
        "exception.go",
        "http.go",
        "interfaces.go",
        "stacktrace.go",
        "writer.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/getsentry/raven-go",
    importpath = "github.com/getsentry/raven-go",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/certifi/gocertifi:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/pkg/errors:go_default_library",
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
