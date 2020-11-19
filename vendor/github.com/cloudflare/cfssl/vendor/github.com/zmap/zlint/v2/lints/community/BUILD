load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "lint_ian_bare_wildcard.go",
        "lint_ian_dns_name_includes_null_char.go",
        "lint_ian_dns_name_starts_with_period.go",
        "lint_ian_iana_pub_suffix_empty.go",
        "lint_ian_wildcard_not_first.go",
        "lint_is_redacted_cert.go",
        "lint_issuer_dn_leading_whitespace.go",
        "lint_issuer_dn_trailing_whitespace.go",
        "lint_issuer_multiple_rdn.go",
        "lint_rsa_exp_negative.go",
        "lint_rsa_no_public_key.go",
        "lint_san_bare_wildcard.go",
        "lint_san_dns_name_duplicate.go",
        "lint_san_dns_name_includes_null_char.go",
        "lint_san_dns_name_starts_with_period.go",
        "lint_san_iana_pub_suffix_empty.go",
        "lint_san_wildcard_not_first.go",
        "lint_subject_dn_leading_whitespace.go",
        "lint_subject_dn_trailing_whitespace.go",
        "lint_subject_multiple_rdn.go",
        "lint_validity_time_not_positive.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/community",
    importpath = "github.com/zmap/zlint/v2/lints/community",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509/pkix:go_default_library",
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
