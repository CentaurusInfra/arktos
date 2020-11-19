load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "addons.go",
        "binding.go",
        "codecs.go",
        "command.go",
        "common.go",
        "consts.go",
        "init.go",
        "mysql.go",
        "packet.go",
        "paramvalue.go",
        "passwd.go",
        "prepared.go",
        "result.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/ziutek/mymysql/native",
    importpath = "github.com/ziutek/mymysql/native",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/cloudflare/cfssl/vendor/github.com/ziutek/mymysql/mysql:go_default_library"],
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
