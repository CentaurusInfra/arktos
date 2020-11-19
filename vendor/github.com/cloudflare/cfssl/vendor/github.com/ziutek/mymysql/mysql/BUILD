load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "errors.go",
        "field.go",
        "interface.go",
        "row.go",
        "types.go",
        "utils.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/ziutek/mymysql/mysql",
    importpath = "github.com/ziutek/mymysql/mysql",
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
