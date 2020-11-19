load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "template.go",
        "unsafe.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/valyala/fasttemplate",
    importpath = "github.com/valyala/fasttemplate",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/cloudflare/cfssl/vendor/github.com/valyala/bytebufferpool:go_default_library"],
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
