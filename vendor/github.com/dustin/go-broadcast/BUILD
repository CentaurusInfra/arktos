load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "broadcaster.go",
        "mux_observer.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/github.com/dustin/go-broadcast",
    importpath = "github.com/dustin/go-broadcast",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "broadcaster_test.go",
        "example_test.go",
        "mux_observer_test.go",
    ],
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
