load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "array.go",
        "buf.go",
        "conn.go",
        "conn_go18.go",
        "connector.go",
        "copy.go",
        "doc.go",
        "encode.go",
        "error.go",
        "notify.go",
        "rows.go",
        "ssl.go",
        "ssl_permissions.go",
        "ssl_windows.go",
        "url.go",
        "user_posix.go",
        "user_windows.go",
        "uuid.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/lib/pq",
    importpath = "github.com/lib/pq",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq/oid:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq/scram:go_default_library",
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
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq/oid:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/lib/pq/scram:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
