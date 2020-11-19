load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["passvault.go"],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/passvault",
    importpath = "github.com/cloudflare/redoctober/passvault",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/ecdh:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/padding:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/symcrypt:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/golang.org/x/crypto/scrypt:go_default_library",
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
