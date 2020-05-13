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
        "//staging/src/k8s.io/arktos-ext/pkg/apis/arktosextensions/v1:all-srcs",
        "//staging/src/k8s.io/arktos-ext/pkg/generated/clientset/versioned:all-srcs",
        "//staging/src/k8s.io/arktos-ext/pkg/generated/informers/externalversions:all-srcs",
        "//staging/src/k8s.io/arktos-ext/pkg/generated/listers/arktosextensions/v1:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
