load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["cfssl-certinfo.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/cmd/cfssl-certinfo",
    importpath = "github.com/cloudflare/cfssl/cmd/cfssl-certinfo",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/cli:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/cli/certinfo:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/config:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/go-sql-driver/mysql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/mattn/go-sqlite3:go_default_library",
    ],
)

go_binary(
    name = "cfssl-certinfo",
    embed = [":go_default_library"],
    tags = ["manual"],
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
