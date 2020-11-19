load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["crypto.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/scan/crypto",
    importpath = "github.com/cloudflare/cfssl/scan/crypto",
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
    srcs = [
        ":package-srcs",
        "//vendor/github.com/cloudflare/cfssl/scan/crypto/md5:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/scan/crypto/rsa:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/scan/crypto/sha1:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/scan/crypto/sha256:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/scan/crypto/sha512:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/scan/crypto/tls:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
