/*
Copyright 2021 Authors of Arktos.

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
var ERROR_IMAGE_NOT_FOUND = fmt.Errorf("image not found")

// Arktos doesn't have its own image registry
// this is simulate the read-only image service to get test images for VM
// the ImageRef is still the original site from the image providers
type ImageType struct {
	Id       int    `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	ImageRef string `json:"imageRef,omitempty"`
}

// for 130 release, only READ operation with static list of images
func initImagesCache() {
	images = make(map[string]ImageType)
	images["ubuntu-xenial"] = ImageType{1, "ubuntu-xenial", "cloud-images.ubuntu.com/xenial/current/xenial-server-cloudimg-amd64-disk1.img"}
	images["cirros-0.5.1"] = ImageType{2, "cirros-0.5.1", "download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img"}

	imageList = make([]*ImageType, len(images))
	i := 0
	for _, v := range images {
		temp := v
		imageList[i] = &temp
		i++
	}
}

func GetImage(name string) (*ImageType, error) {
	if image, found := images[name]; found {
		return &image, nil
	}

	return nil, ERROR_IMAGE_NOT_FOUND
}

func ListImages() []*ImageType {
	return imageList
}
