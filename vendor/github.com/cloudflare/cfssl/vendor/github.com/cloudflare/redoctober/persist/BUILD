load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "file.go",
        "null.go",
        "persist.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/persist",
    importpath = "github.com/cloudflare/redoctober/persist",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/keycache:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/passvault:go_default_library",
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
