load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "experimental.go",
        "gce_windows_util.go",
        "prometheus.go",
        "util.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/prometheus",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/pkg/config:go_default_library",
        "//perf-tests/clusterloader2/pkg/flags:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework/client:go_default_library",
        "//perf-tests/clusterloader2/pkg/util:go_default_library",
        "//pkg/util/system:go_default_library",
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/api/rbac/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/golang.org/x/sync/errgroup:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["util_test.go"],
    embed = [":go_default_library"],
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
