load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["core.go"],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/core",
    importpath = "github.com/cloudflare/redoctober/core",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/cryptor:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/hipchat:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/keycache:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/order:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/passvault:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/persist:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/report:go_default_library",
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
