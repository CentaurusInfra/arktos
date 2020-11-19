load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "nilref_nil_darwin.go",
        "nilref_zero_darwin.go",
        "root.go",
        "root_bsd.go",
        "root_cgo_darwin.go",
        "root_darwin.go",
        "root_darwin_armx.go",
        "root_linux.go",
        "root_nacl.go",
        "root_nocgo_darwin.go",
        "root_plan9.go",
        "root_solaris.go",
        "root_unix.go",
        "root_windows.go",
    ],
    cgo = True,
    clinkopts = select({
        "@io_bazel_rules_go//go/platform:darwin_386": [
            "-framework CoreFoundation -framework Security",
        ],
        "@io_bazel_rules_go//go/platform:darwin_amd64": [
            "-framework CoreFoundation -framework Security",
        ],
        "//conditions:default": [],
    }),
    copts = select({
        "@io_bazel_rules_go//go/platform:darwin_386": [
            "-mmacosx-version-min=10.6 -D__MAC_OS_X_VERSION_MAX_ALLOWED=1060",
        ],
        "@io_bazel_rules_go//go/platform:darwin_amd64": [
            "-mmacosx-version-min=10.6 -D__MAC_OS_X_VERSION_MAX_ALLOWED=1060",
        ],
        "//conditions:default": [],
    }),
    importmap = "k8s.io/kubernetes/vendor/github.com/cloudflare/cfssl/transport/roots/system",
    importpath = "github.com/cloudflare/cfssl/transport/roots/system",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["root_darwin_test.go"],
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
