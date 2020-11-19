load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["signhandler.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/api/signhandler",
    importpath = "github.com/cloudflare/cfssl/api/signhandler",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/api:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/auth:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/bundler:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/errors:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["signhandler_test.go"],
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/api:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb/sql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb/testdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer/local:go_default_library",
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
