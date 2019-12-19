load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "bcast.go",
        "priority_queue.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/grafov/bcast",
    importpath = "github.com/grafov/bcast",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["bcast_test.go"],
    embed = [":go_default_library"],
    tags = ["manual"],
    deps = ["//vendor/github.com/grafov/bcast/vendor/gopkg.in/fatih/set.v0:go_default_library"],
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
        "//vendor/github.com/grafov/bcast/vendor/gopkg.in/fatih/set.v0:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
