load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "lookup.go",
        "whitelist.go",
        "whitelist_net.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/whitelist",
    importpath = "github.com/cloudflare/cfssl/whitelist",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "http_test.go",
        "whitelist_net_test.go",
        "whitelist_test.go",
    ],
    embed = [":go_default_library"],
    tags = ["manual"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//vendor/github.com/cloudflare/cfssl/whitelist/example:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
