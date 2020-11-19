load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "sha256.go",
        "sha256block.go",
        "sha256block_386.s",
        "sha256block_amd64.s",
        "sha256block_decl.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/scan/crypto/sha256",
    importpath = "github.com/cloudflare/cfssl/scan/crypto/sha256",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["sha256_test.go"],
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
