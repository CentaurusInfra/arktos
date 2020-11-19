load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "dbconf.go",
        "dialect.go",
        "migrate.go",
        "migration_go.go",
        "migration_sql.go",
        "util.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/bitbucket.org/liamstask/goose/lib/goose",
    importpath = "bitbucket.org/liamstask/goose/lib/goose",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/go-sql-driver/mysql:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/kylelemons/go-gypsy/yaml:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/mattn/go-sqlite3:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/ziutek/mymysql/godrv:go_default_library",
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
