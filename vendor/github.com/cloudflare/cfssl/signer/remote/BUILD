load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["remote.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/signer/remote",
    importpath = "github.com/cloudflare/cfssl/signer/remote",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/api/client:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/certdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/errors:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/info:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["remote_test.go"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/api/info:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/api/signhandler:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers/testsuite:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/info:go_default_library",
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
