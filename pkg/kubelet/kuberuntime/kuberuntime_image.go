/*
Copyright 2016 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package kuberuntime

import (
	"fmt"
	"k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/credentialprovider"
	credentialprovidersecrets "k8s.io/kubernetes/pkg/credentialprovider/secrets"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/util/parsers"
	"strings"
)

// PullImage pulls an image from the network to local storage using the supplied
// secrets if necessary.
// TODO: interface to check if image exists on a service
//       maybe more straitforward way to iterate on each image services
func (m *kubeGenericRuntimeManager) PullImage(image kubecontainer.ImageSpec, pullSecrets []v1.Secret, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	img := image.Image
	repoToPull, _, _, err := parsers.ParseImageName(img)
	if err != nil {
		return "", err
	}

	keyring, err := credentialprovidersecrets.MakeDockerKeyring(pullSecrets, m.keyring)
	if err != nil {
		return "", err
	}

	var imageServices []internalapi.ImageManagerService

	// Get the image service from the map cache for this image
	imageWithoutTag := strings.Split(image.Image, ":")[0]
	if is := m.getPodImageService(imageWithoutTag); is != nil {
		klog.Infof("Got desired image service for image %v, %v", image, is)
		imageServices = []internalapi.ImageManagerService{is}
	} else {
		klog.Infof("Cannot find desired image service for image %s, loop through all image services to pull", image.Image)
		imageServices, err = m.GetAllImageServices()
		if err != nil {
			klog.Errorf("GetAllImageServices Failed: %v", err)
			return "", err
		}
	}

	imgSpec := &runtimeapi.ImageSpec{Image: img}
	creds, withCredentials := keyring.Lookup(repoToPull)
	if !withCredentials {
		klog.V(3).Infof("Pulling image %q without credentials", img)

		for idx, imageService := range imageServices {
			klog.Infof("Pulling image with image service %d:%v", idx, imageService)
			imageRef, err := imageService.PullImage(imgSpec, nil, podSandboxConfig)
			if err != nil {
				klog.Errorf("Pull image %q failed: %v", img, err)
			} else {
				klog.Infof("Got image ref for image %q: %v", image, imageRef)
				return imageRef, nil
			}
		}
	}

	var pullErrs []error
	for _, currentCreds := range creds {
		authConfig := credentialprovider.LazyProvide(currentCreds, repoToPull)
		auth := &runtimeapi.AuthConfig{
			Username:      authConfig.Username,
			Password:      authConfig.Password,
			Auth:          authConfig.Auth,
			ServerAddress: authConfig.ServerAddress,
			IdentityToken: authConfig.IdentityToken,
			RegistryToken: authConfig.RegistryToken,
		}

		for idx, imageService := range imageServices {
			klog.Infof("Pulling image with image service %d:%v", idx, imageService)
			imageRef, err := imageService.PullImage(imgSpec, auth, podSandboxConfig)
			// If there was no error, return success
			if err == nil {
				klog.Infof("Got image ref for image %q: %v", image, imageRef)
				return imageRef, nil
			}
			klog.Errorf("Pull image %q failed: %v", img, err)
			pullErrs = append(pullErrs, err)
		}
	}

	return "", utilerrors.NewAggregate(pullErrs)
}

// GetImageRef gets the ID of the image which has already been in
// the local storage. It returns ("", nil) if the image isn't in the local storage.
func (m *kubeGenericRuntimeManager) GetImageRef(image kubecontainer.ImageSpec) (string, error) {

	// Get the image service from the map cache for this image
	var imageServices []internalapi.ImageManagerService
	var err error
	// Get the image service from the map cache for this image
	imageWithoutTag := strings.Split(image.Image, ":")[0]
	if is := m.getPodImageService(imageWithoutTag); is != nil {
		klog.Infof("Got desired image service for image %v, %v", image, is)
		imageServices = []internalapi.ImageManagerService{is}
	} else {
		klog.Infof("Cannot find desired image service for image %s, loop through all image services to pull", image.Image)
		imageServices, err = m.GetAllImageServices()
		if err != nil {
			klog.Errorf("GetAllImageServices Failed: %v", err)
			return "", err
		}
	}

	for _, imageService := range imageServices {
		status, err := imageService.ImageStatus(&runtimeapi.ImageSpec{Image: image.Image})
		if err != nil || status == nil {
			klog.Infof("ImageStatus for image %q failed: %v", image, err)
		} else {
			klog.Errorf("Got ImageStatus for image %q: %v", image, status)
			return status.Id, nil
		}

	}

	// if not found, return "", nil
	return "", nil
}

// ListImages gets all images currently on the machine.
// TODO: what if the dup image from different imageService ?
//
func (m *kubeGenericRuntimeManager) ListImages() ([]kubecontainer.Image, error) {
	var images []kubecontainer.Image

	imageServices, err := m.GetAllImageServices()
	if err != nil {
		klog.Errorf("GetAllImageServices Failed: %v", err)
		return nil, err
	}

	for _, imageService := range imageServices {
		allImages, err := imageService.ListImages(nil)
		if err != nil {
			klog.Errorf("ListImages failed: %v", err)
			return nil, err
		}

		for _, img := range allImages {
			images = append(images, kubecontainer.Image{
				ID:          img.Id,
				Size:        int64(img.Size_),
				RepoTags:    img.RepoTags,
				RepoDigests: img.RepoDigests,
			})
		}
	}

	return images, nil
}

// RemoveImage removes the specified image.
// TODO: implement a imageExists interface method here
//
func (m *kubeGenericRuntimeManager) RemoveImage(image kubecontainer.ImageSpec) error {

	imageServices, err := m.GetAllImageServices()
	if err != nil {
		klog.Errorf("GetAllImageServices Failed: %v", err)
		return err
	}

	for _, imageService := range imageServices {
		err := imageService.RemoveImage(&runtimeapi.ImageSpec{Image: image.Image})
		// ignore the error and continue to next imageServices
		//
		if err != nil {
			klog.Errorf("Remove image %q failed: %v", image.Image, err)

		} else {
			// delete entry from the cache
			m.removePodImageService(image.Image)
			return nil
		}
	}

	// return error if reaching here
	return fmt.Errorf("image %q not found", image.Image)
}

// ImageStats returns the statistics of the image.
// Notice that current logic doesn't really work for images which share layers (e.g. docker image),
// this is a known issue, and we'll address this by getting imagefs stats directly from CRI.
// TODO: Get imagefs stats directly from CRI.
// TODO: the ImageStates should be per imageService
//
func (m *kubeGenericRuntimeManager) ImageStats() (*kubecontainer.ImageStats, error) {
	imageServices, err := m.GetAllImageServices()
	if err != nil {
		klog.Errorf("GetAllImageServices Failed: %v", err)
		return nil, err
	}

	stats := &kubecontainer.ImageStats{}
	for _, imageService := range imageServices {
		allImages, err := imageService.ListImages(nil)
		if err != nil {
			klog.Errorf("ListImages failed: %v", err)
			return nil, err
		}

		for _, img := range allImages {
			stats.TotalStorageBytes += img.Size_
		}
	}
	return stats, nil
}
