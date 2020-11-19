load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "int.go",
        "int16.go",
        "int32.go",
        "int64.go",
        "int8.go",
        "uint.go",
        "uint16.go",
        "uint32.go",
        "uint64.go",
        "uint8.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.incremental",
    importpath = "github.com/GeertJohan/go.incremental",
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
