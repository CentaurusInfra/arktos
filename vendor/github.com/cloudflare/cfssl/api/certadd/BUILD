load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["insert.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/api/certadd",
    importpath = "github.com/cloudflare/cfssl/api/certadd",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/api:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/errors:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/ocsp:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/golang.org/x/crypto/ocsp:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["insert_test.go"],
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/certdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb/sql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb/testdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/ocsp:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/golang.org/x/crypto/ocsp:go_default_library",
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
