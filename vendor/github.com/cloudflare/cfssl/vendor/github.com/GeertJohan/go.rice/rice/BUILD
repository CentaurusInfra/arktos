load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "append.go",
        "clean.go",
        "embed-go.go",
        "embed-syso.go",
        "find.go",
        "flags.go",
        "identifier.go",
        "main.go",
        "templates.go",
        "util.go",
        "writecoff.go",
    ],
    importmap = "github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.rice/rice",
    importpath = "github.com/GeertJohan/go.rice/rice",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.incremental:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/GeertJohan/go.rice/embedded:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/akavel/rsrc/binutil:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/akavel/rsrc/coff:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/daaku/go.zipexe:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/jessevdk/go-flags:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/nkovacs/streamquote:go_default_library",
        "//vendor/github.com/cloudflare/cfssl/vendor/github.com/valyala/fasttemplate:go_default_library",
    ],
)

go_binary(
    name = "rice",
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
