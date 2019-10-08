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
        "//pkg/cloudfabric-controller/api/types/v1alpha1:all-srcs",
        "//pkg/cloudfabric-controller/controllerframework:all-srcs",
        "//pkg/cloudfabric-controller/replicaset:all-srcs",
        "//pkg/cloudfabric-controller/testutil:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
