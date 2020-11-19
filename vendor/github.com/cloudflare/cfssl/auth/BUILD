load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["auth.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/auth",
    importpath = "github.com/cloudflare/cfssl/auth",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["auth_test.go"],
    data = glob(["testdata/**"]),
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
