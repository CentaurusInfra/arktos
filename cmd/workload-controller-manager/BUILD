load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "k8s.io/kubernetes/cmd/workload-controller-manager",
    visibility = ["//visibility:private"],
    deps = [
        "//cmd/workload-controller-manager/app:go_default_library",
        "//cmd/workload-controller-manager/app/config:go_default_library",
        "//pkg/client/metrics/prometheus:go_default_library",
        "//pkg/master/ports:go_default_library",
        "//pkg/version/prometheus:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/server/options:go_default_library",
        "//staging/src/k8s.io/client-go/tools/clientcmd:go_default_library",
        "//staging/src/k8s.io/component-base/logs:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_binary(
    name = "workload-controller-manager",
    embed = [":go_default_library"],
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
    srcs = [
        ":package-srcs",
        "//cmd/workload-controller-manager/app:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
