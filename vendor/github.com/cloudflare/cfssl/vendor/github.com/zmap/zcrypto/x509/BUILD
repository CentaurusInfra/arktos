load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "cert_pool.go",
        "certificate_type.go",
        "chain.go",
        "extended_key_usage.go",
        "extensions.go",
        "fingerprint.go",
        "generated_certvalidationlevel_string.go",
        "json.go",
        "names.go",
        "pem_decrypt.go",
        "pkcs1.go",
        "pkcs8.go",
        "sec1.go",
        "tor_service_descriptor.go",
        "validation.go",
        "verify.go",
        "x509.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509",
    importpath = "github.com/zmap/zcrypto/x509",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/weppos/publicsuffix-go/publicsuffix:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/json:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/util:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509/ct:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509/pkix:go_default_library",
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
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509/ct:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509/pkix:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
