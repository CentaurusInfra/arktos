load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "set.go",
        "set_nots.go",
        "set_ts.go",
    ],
    importmap = "github.com/grafov/bcast/vendor/gopkg.in/fatih/set.v0",
    importpath = "gopkg.in/fatih/set.v0",
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
