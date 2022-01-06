/*
Copyright 2021 The Arktos Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openstack

import "fmt"

var images = map[string]ImageType{}
var imageList = []*ImageType{}

// Arktos doesn't have its own image registry
// this is simulate the read-only image service to get test images for VM
// the ImageRef is still the original site from the image providers
type ImageType struct {
	Id       int
	Name     string
	ImageRef string
}

// for 130 release, only READ operation with static list of images
func initImagesCache() {
	images = make(map[string]ImageType)
	images["ubuntu-xenial"] = ImageType{1, "ubuntu-xenial", "cloud-images.ubuntu.com/xenial/current/xenial-server-cloudimg-amd64-disk1.img"}
	images["cirros-0.5.1"] = ImageType{2, "cirros-0.5.1", "download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img"}

	imageList = make([]*ImageType, len(flavors))
	i := 0
	for _, v := range images {
		imageList[i] = &v
	}
}

func GetImage(name string) (*ImageType, error) {
	if image, found := images[name]; found {
		return &image, nil
	}

	return nil, fmt.Errorf("image %s, not found", name)
}

func ListImages() []*ImageType {
	return imageList
}

