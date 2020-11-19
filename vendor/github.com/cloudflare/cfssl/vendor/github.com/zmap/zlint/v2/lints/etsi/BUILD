load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "lint_qcstatem_etsi_present_qcs_critical.go",
        "lint_qcstatem_etsi_type_as_statem.go",
        "lint_qcstatem_mandatory_etsi_statems.go",
        "lint_qcstatem_qccompliance_valid.go",
        "lint_qcstatem_qclimitvalue_valid.go",
        "lint_qcstatem_qcpds_lang_case.go",
        "lint_qcstatem_qcpds_valid.go",
        "lint_qcstatem_qcretentionperiod_valid.go",
        "lint_qcstatem_qcsscd_valid.go",
        "lint_qcstatem_qctype_valid.go",
        "lint_qcstatem_qctype_web.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/zmap/zlint/v2/lints/etsi",
    importpath = "github.com/zmap/zlint/v2/lints/etsi",
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
