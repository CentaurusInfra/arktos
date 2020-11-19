load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "appended.go",
        "box.go",
        "config.go",
        "debug.go",
        "embedded.go",
        "file.go",
        "http.go",
        "sort.go",
        "virtual.go",
        "walk.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.rice",
    importpath = "github.com/GeertJohan/go.rice",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.rice/embedded:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/daaku/go.zipexe:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.rice/embedded:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.rice/rice:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
