load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "md5.go",
        "md5block.go",
        "md5block_386.s",
        "md5block_amd64.s",
        "md5block_amd64p32.s",
        "md5block_arm.s",
        "md5block_decl.go",
        "md5block_generic.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/scan/crypto/md5",
    importpath = "github.com/cloudflare/cfssl/scan/crypto/md5",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "example_test.go",
        "md5_test.go",
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
