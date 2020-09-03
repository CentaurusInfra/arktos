load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "global_qps_load.go",
        "interface.go",
        "parallelism_limited.go",
        "qps_load.go",
        "randomized_load.go",
        "randomized_time_limited.go",
        "simple_tuning_set_factory.go",
        "stepped_load.go",
        "time_limited.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/tuningset",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/api:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/client-go/util/workqueue:go_default_library",
        "//vendor/golang.org/x/time/rate:go_default_library",
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
