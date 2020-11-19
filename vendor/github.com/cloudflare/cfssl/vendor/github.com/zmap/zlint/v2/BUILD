load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "resultset.go",
        "zlint.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2",
    importpath = "github.com/zmap/zlint/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lint:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/apple:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/cabf_br:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/cabf_ev:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/community:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/etsi:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/mozilla:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/rfc:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lint:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/apple:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/cabf_br:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/cabf_ev:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/community:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/etsi:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/mozilla:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/rfc:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/util:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
