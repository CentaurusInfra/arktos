load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["local.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/signer/local",
    importpath = "github.com/cloudflare/cfssl/signer/local",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/certdb:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/errors:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/info:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/google/certificate-transparency-go:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/google/certificate-transparency-go/client:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/google/certificate-transparency-go/jsonclient:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lint:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/golang.org/x/net/context:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["local_test.go"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/csr:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/errors:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/helpers:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/signer:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/google/certificate-transparency-go:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lint:go_default_library",
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
