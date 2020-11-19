load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "base.go",
        "registration.go",
        "result.go",
        "source.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lint",
    importpath = "github.com/zmap/zlint/v2/lint",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/util:go_default_library",
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
