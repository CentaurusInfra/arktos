# Copyright 2016 The Kubernetes Authors.
# Copyright 2020 Authors of Arktos - file modified.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the etcd image
#
# Usage:
# 	[BUNDLED_ETCD_VERSIONS=3.4.3.0] [REGISTRY=k8s.gcr.io] [ARCH=amd64] [BASEIMAGE=busybox] make (build|push)
#
# The image contains different etcd versions to simplify
# upgrades. Thus be careful when removing any versions from here.
#
# NOTE: The etcd upgrade rules are that you can upgrade only 1 minor
# version at a time, and patch release don't matter.
#
# Except from etcd-$(version) and etcdctl-$(version) binaries, we also
# need etcd and etcdctl binaries for backward compatibility reasons.
# That binary will be set to the last version from $(BUNDLED_ETCD_VERSIONS).
BUNDLED_ETCD_VERSIONS?=3.4.3

# LATEST_ETCD_VERSION identifies the most recent etcd version available.
LATEST_ETCD_VERSION?=3.4.3

# ETCD branch to build etcd binaries
ETCD_BRANCH?=release-3.4-ETCDPartition-RevGeneration

# REVISION provides a version number fo this image and all it's bundled
# artifacts. It should start at zero for each LATEST_ETCD_VERSION and increment
# for each revision of this image at that etcd version.
REVISION?=1

# IMAGE_TAG Uniquely identifies k8s.gcr.io/etcd docker image with a tag of the form "<etcd-version>-<revision>".
IMAGE_TAG=$(LATEST_ETCD_VERSION).0

ARCH?=amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x
# Image should be pulled from k8s.gcr.io, which will auto-detect
# region (us, eu, asia, ...) and pull from the closest.
REGISTRY?=gcr.io
# Images should be pushed to staging-k8s.gcr.io.
PUSH_REGISTRY?=gcr.io

MANIFEST_IMAGE := etcd

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled
# golang version should match the golang version from https://github.com/coreos/etcd/releases for the current ETCD_VERSION.
GOLANG_VERSION?=1.12.9
GOARM=7
TEMP_DIR:=$(shell mktemp -d)

ifeq ($(ARCH),amd64)
    BASEIMAGE?=k8s.gcr.io/debian-base:v1.0.0
endif
ifeq ($(ARCH),arm)
    BASEIMAGE?=k8s.gcr.io/debian-base-arm:v1.0.0
endif
ifeq ($(ARCH),arm64)
    BASEIMAGE?=k8s.gcr.io/debian-base-arm64:v1.0.0
endif
ifeq ($(ARCH),ppc64le)
    BASEIMAGE?=k8s.gcr.io/debian-base-ppc64le:v1.0.0
endif
ifeq ($(ARCH),s390x)
    BASEIMAGE?=k8s.gcr.io/debian-base-s390x:v1.0.0
endif

build:
	# Copy the content in this dir to the temp dir,
	# without copying the subdirectories.
	find ./ -maxdepth 1 -type f | xargs -I {} cp {} $(TEMP_DIR)


ifeq ($(ARCH),amd64)

	# Do not compile if we should make an image for amd64, use the official etcd binaries instead
	# For each release create a tmp dir 'etcd_release_tmp_dir' and unpack the release tar there.
	for version in $(BUNDLED_ETCD_VERSIONS); do \
		etcd_release_tmp_dir=$(shell mktemp -d); \
		curl -sSL --retry 5 https://github.com/centaurus-cloud/etcd/releases/download/v$$version/etcd-v$$version-linux-amd64.tar.gz | tar -xz -C $$etcd_release_tmp_dir --strip-components=1; \
		cp $$etcd_release_tmp_dir/etcd $$etcd_release_tmp_dir/etcdctl $(TEMP_DIR)/; \
		cp $(TEMP_DIR)/etcd $(TEMP_DIR)/etcd-$$version; \
		cp $(TEMP_DIR)/etcdctl $(TEMP_DIR)/etcdctl-$$version; \
	done
else

	# Download etcd in a golang container and cross-compile it statically
	# For each release create a tmp dir 'etcd_release_tmp_dir' and unpack the release tar there.
        arch_prefix=""
        ifeq ($(ARCH),arm)
		arch_prefix="GOARM=$(GOARM)"
        endif

        # use '/go/src/go.etcd.io/etcd' to build etcd 3.4 and later.
	for version in $(BUNDLED_ETCD_VERSIONS); do \
	        etcd_release_tmp_dir=$(shell mktemp -d); \
		etcd_build_dir="/go/src/github.com/coreos/etcd"; \
		if [ $$(echo $$version | cut -d. -f2) -gt 3 ]; then \
			etcd_build_dir="/go/src/go.etcd.io/etcd"; \
		fi; \
		docker run --interactive -v $${etcd_release_tmp_dir}:/etcdbin golang:$(GOLANG_VERSION)$(DOCKER_VOL_OPTS) /bin/bash -c \
			"git clone --single-branch --branch $(ETCD_BRANCH) https://github.com/centaurus-cloud/etcd.git $$etcd_build_dir \
			&& cd $$etcd_build_dir \
			&& git checkout v$${version} \
			&& $(arch_prefix) GOARCH=$(ARCH) ./build \
			&& cp -f bin/$(ARCH)/etcd* bin/etcd* /etcdbin; echo 'done'"; \
		cp $$etcd_release_tmp_dir/etcd $$etcd_release_tmp_dir/etcdctl $(TEMP_DIR)/; \
		cp $(TEMP_DIR)/etcd $(TEMP_DIR)/etcd-$$version; \
		cp $(TEMP_DIR)/etcdctl $(TEMP_DIR)/etcdctl-$$version; \
	done

	# Add this ENV variable in order to workaround an unsupported arch blocker
	# The multiarch feature is in an limited and experimental state right now, and etcd should work fine on arm64
	# On arm (which is 32-bit), it can't handle >1GB data in-memory, but it is very unlikely someone tinkering with their limited arm devices would reach such a high usage
	# ppc64le is still quite untested, but compiles and is probably in the process of being validated by IBM.
	cd $(TEMP_DIR) && echo "ENV ETCD_UNSUPPORTED_ARCH=$(ARCH)" >> Dockerfile
endif
push:
	docker build --pull -t $(REGISTRY)/etcd-$(ARCH):$(IMAGE_TAG)  .
	docker tag $(REGISTRY)/etcd-$(ARCH):$(IMAGE_TAG) $(REGISTRY)/$(MANIFEST_IMAGE)-$(ARCH):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(MANIFEST_IMAGE)-$(ARCH):$(IMAGE_TAG)
sub-build-%:
	$(MAKE) ARCH=$* build

all-build: $(addprefix sub-build-,$(ALL_ARCH))

sub-push-image-%:
	$(MAKE) ARCH=$* push

all-push-images: $(addprefix sub-push-image-,$(ALL_ARCH))

all-push: all-push-images push-manifest

push-manifest:
	docker manifest create --amend $(MANIFEST_IMAGE):$(IMAGE_TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(MANIFEST_IMAGE)\-&:$(IMAGE_TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${MANIFEST_IMAGE}:${IMAGE_TAG} ${MANIFEST_IMAGE}-$${arch}:${IMAGE_TAG}; done
	docker manifest push --purge ${MANIFEST_IMAGE}:${IMAGE_TAG}

unit-test:
	docker run --interactive -v $(shell pwd)/../../../:/go/src/k8s.io/kubernetes -e GOARCH=$(ARCH) golang:$(GOLANG_VERSION) \
		/bin/bash -c "CGO_ENABLED=0 go test -v k8s.io/kubernetes/cluster/images/etcd/migrate"

# Integration tests require both a golang build environment and all the etcd binaries from a `k8s.gcr.io/etcd` image (`/usr/local/bin/etcd-<version>`, ...).
# Since the `k8s.gcr.io/etcd` image is for runtime only and does not have a build golang environment, we create a new docker image to run integration tests
# with.
build-integration-test-image: build
	cp -r $(TEMP_DIR) $(TEMP_DIR)_integration_test
	cp Dockerfile $(TEMP_DIR)_integration_test/Dockerfile
	cd $(TEMP_DIR)_integration_test && sed -i.bak 's|BASEIMAGE|golang:$(GOLANG_VERSION)|g' Dockerfile
	docker build --pull -t etcd-integration-test $(TEMP_DIR)_integration_test

integration-test:
	docker run --interactive -v $(shell pwd)/../../../:/go/src/k8s.io/kubernetes -e GOARCH=$(ARCH) etcd-integration-test \
		/bin/bash -c "CGO_ENABLED=0 go test -tags=integration k8s.io/kubernetes/cluster/images/etcd/migrate -args -v 10 -logtostderr true"

integration-build-test: build-integration-test-image integration-test
test: unit-test integration-build-test
all: all-build test
.PHONY:	build push push-manifest all-push all-push-images all-build unit-test build-integration-test-image integration-test integration-build-test test
