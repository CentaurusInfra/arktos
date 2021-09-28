load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "etcd_metrics.go",
        "metrics_for_e2e.go",
        "profile.go",
        "resource_usage.go",
        "scheduler_latency.go",
        "scheduling_throughput.go",
        "service_creation_latency.go",
        "sleep.go",
        "system_pod_metrics.go",
        "timer.go",
        "wait_for_controlled_pods.go",
        "wait_for_nodes.go",
        "wait_for_pods.go",
        "wait_for_pvcs.go",
        "wait_for_pvs.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement/common",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/pkg/errors:go_default_library",
        "//perf-tests/clusterloader2/pkg/execservice:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework/client:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/checker:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/gatherers:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/informer:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/runtimeobjects:go_default_library",
        "//perf-tests/clusterloader2/pkg/measurement/util/workerqueue:go_default_library",
        "//perf-tests/clusterloader2/pkg/util:go_default_library",
        "//pkg/master/ports:go_default_library",
        "//pkg/scheduler/metrics:go_default_library",
        "//pkg/util/system:go_default_library",
        "//staging/src/k8s.io/api/core/v1:go_default_library",
        "//staging/src/k8s.io/api/rbac/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/api/equality:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/api/meta:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//staging/src/k8s.io/client-go/tools/cache:go_default_library",
        "//test/e2e/framework/metrics:go_default_library",
        "//vendor/github.com/prometheus/common/model:go_default_library",
        "//vendor/gopkg.in/yaml.v2:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["system_pod_metrics_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//perf-tests/clusterloader2/pkg/measurement:go_default_library",
        "//vendor/gopkg.in/yaml.v2:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/common/bundle:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/common/probes:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/common/slos:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
