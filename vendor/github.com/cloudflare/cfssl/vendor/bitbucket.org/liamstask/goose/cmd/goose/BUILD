load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "cmd.go",
        "cmd_create.go",
        "cmd_dbversion.go",
        "cmd_down.go",
        "cmd_redo.go",
        "cmd_status.go",
        "cmd_up.go",
        "main.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/bitbucket.org/liamstask/goose/cmd/goose",
    importpath = "bitbucket.org/liamstask/goose/cmd/goose",
    visibility = ["//visibility:private"],
    deps = ["//vendor/github.com/cloudflare/cfssl/vendor/bitbucket.org/liamstask/goose/lib/goose:go_default_library"],
)

go_binary(
    name = "goose",
    embed = [":go_default_library"],
    tags = ["manual"],
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
