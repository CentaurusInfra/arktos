load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["config.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/multiroot/config",
    importpath = "github.com/cloudflare/cfssl/multiroot/config",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/certdb/dbconf:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers/derhelpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/client:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/redoctober/core:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/jmoiron/sqlx:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/whitelist:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["config_test.go"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/mattn/go-sqlite3:go_default_library",
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
