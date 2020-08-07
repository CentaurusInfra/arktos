load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "histogram.go",
        "latency_metric.go",
        "object_store.go",
        "perftype.go",
        "phase_latency.go",
        "pods.go",
        "prometheus.go",
        "pvcs.go",
        "pvs.go",
        "resource.go",
        "selector.go",
        "ssh.go",
        "wait_for_nodes.go",
        "wait_for_pods.go",
        "wait_for_pvcs.go",
        "wait_for_pvs.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement/util",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/pkg/util:go_default_library",
        "//pkg/ssh:go_default_library",
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//staging/src/k8s.io/client-go/tools/cache:go_default_library",
        "//staging/src/k8s.io/component-base/metrics/testutil:go_default_library",
        "//vendor/github.com/prometheus/client_model/go:go_default_library",
        "//vendor/github.com/prometheus/common/expfmt:go_default_library",
        "//vendor/github.com/prometheus/common/model:go_default_library",
        "//vendor/golang.org/x/crypto/ssh:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["histogram_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/github.com/prometheus/common/model:go_default_library"],
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
        "//perf-tests/clusterloader2/pkg/measurement/util/checker:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/util/gatherers:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/util/informer:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/util/kubelet:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/util/kubemark:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/util/runtimeobjects:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/util/workerqueue:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
