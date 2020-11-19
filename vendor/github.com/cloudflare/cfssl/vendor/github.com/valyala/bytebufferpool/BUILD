load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "bytebuffer.go",
        "doc.go",
        "pool.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/valyala/bytebufferpool",
    importpath = "github.com/valyala/bytebufferpool",
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
