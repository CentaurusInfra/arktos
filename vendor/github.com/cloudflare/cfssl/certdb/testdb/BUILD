load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["testdb.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/certdb/testdb",
    importpath = "github.com/cloudflare/cfssl/certdb/testdb",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/go-sql-driver/mysql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/jmoiron/sqlx:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/mattn/go-sqlite3:go_default_library",
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
