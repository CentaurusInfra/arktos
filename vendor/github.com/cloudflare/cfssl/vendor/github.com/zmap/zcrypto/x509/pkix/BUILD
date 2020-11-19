load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "json.go",
        "oid.go",
        "oid_names.go",
        "pkix.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509/pkix",
    importpath = "github.com/zmap/zcrypto/x509/pkix",
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
