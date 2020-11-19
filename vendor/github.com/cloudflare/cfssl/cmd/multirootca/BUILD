load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "api.go",
        "ca.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/cmd/multirootca",
    importpath = "github.com/cloudflare/cfssl/cmd/multirootca",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/api:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/api/info:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/auth:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb/sql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/multiroot/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer/local:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/go-metrics:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/go-sql-driver/mysql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/mattn/go-sqlite3:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/whitelist:go_default_library",
    ],
)

go_binary(
    name = "multirootca",
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
