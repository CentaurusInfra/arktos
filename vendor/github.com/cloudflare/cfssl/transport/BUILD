load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "client.go",
        "doc.go",
        "listener.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/transport",
    importpath = "github.com/cloudflare/cfssl/transport",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/csr:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/errors:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/revoke:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/ca:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/core:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/kp:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/roots:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/cloudflare/backoff:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["transport_test.go"],
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/api/client:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/csr:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/info:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/log:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/transport/core:go_default_library",
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
        "//vendor/github.com/cloudflare/cfssl/transport/ca:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/transport/core:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/transport/example/exlib:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/transport/example/maclient:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/transport/example/maserver:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/transport/kp:all-srcs",
        "//vendor/github.com/cloudflare/cfssl/transport/roots:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
