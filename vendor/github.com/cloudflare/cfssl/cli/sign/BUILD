load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["sign.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/cli/sign",
    importpath = "github.com/cloudflare/cfssl/cli/sign",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/certdb/dbconf:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb/sql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/cli:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer/universal:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/jmoiron/sqlx:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["sign_test.go"],
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/certdb/sql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb/testdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/cli:go_default_library",
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
