load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "lint_mp_allowed_eku.go",
        "lint_mp_authority_key_identifier_correct.go",
        "lint_mp_exponent_cannot_be_one.go",
        "lint_mp_modulus_must_be_2048_bits_or_more.go",
        "lint_mp_modulus_must_be_divisible_by_8.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/mozilla",
    importpath = "github.com/zmap/zlint/v2/lints/mozilla",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lint:go_default_library",
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
