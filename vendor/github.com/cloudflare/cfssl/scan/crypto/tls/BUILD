load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "alert.go",
        "cfsslscan_common.go",
        "cfsslscan_handshake.go",
        "cipher_suites.go",
        "common.go",
        "conn.go",
        "handshake_client.go",
        "handshake_messages.go",
        "handshake_server.go",
        "key_agreement.go",
        "prf.go",
        "ticket.go",
        "tls.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/scan/crypto/tls",
    importpath = "github.com/cloudflare/cfssl/scan/crypto/tls",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "conn_test.go",
        "example_test.go",
        "handshake_client_test.go",
        "handshake_messages_test.go",
        "handshake_server_test.go",
        "handshake_test.go",
        "prf_test.go",
        "tls_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    tags = ["manual"],
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
