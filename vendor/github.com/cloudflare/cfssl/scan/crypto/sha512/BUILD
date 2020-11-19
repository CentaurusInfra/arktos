load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "sha512.go",
        "sha512block.go",
        "sha512block_amd64.s",
        "sha512block_decl.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/scan/crypto/sha512",
    importpath = "github.com/cloudflare/cfssl/scan/crypto/sha512",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["sha512_test.go"],
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
