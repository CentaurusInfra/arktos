load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "algorithm_identifier.go",
        "ca.go",
        "countries.go",
        "eku.go",
        "encodings.go",
        "ev.go",
        "fqdn.go",
        "gtld.go",
        "gtld_map.go",
        "ip.go",
        "ku.go",
        "names.go",
        "oid.go",
        "primes.go",
        "qc_stmt.go",
        "rdn.go",
        "time.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/util",
    importpath = "github.com/zmap/zlint/v2/util",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/weppos/publicsuffix-go/publicsuffix:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/util:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/zmap/zcrypto/x509/pkix:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/golang.org/x/crypto/cryptobyte:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/golang.org/x/crypto/cryptobyte/asn1:go_default_library",
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
