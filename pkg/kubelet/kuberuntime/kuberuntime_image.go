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
)

// PullImage pulls an image from the network to local storage using the supplied
// secrets if necessary.
// TODO: interface to check if image exists on a service
//       maybe more straitforward way to iterate on each image services
func (m *kubeGenericRuntimeManager) PullImage(image kubecontainer.ImageSpec, pullSecrets []v1.Secret, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	if image.Pod == nil {
		klog.Errorf("Pod is not set in imageSpec: %v", image)
		return "", fmt.Errorf("POD is not set in imageSpec")
	}

	img := image.Image
	repoToPull, _, _, err := parsers.ParseImageName(img)
	if err != nil {
		return "", err
	}

	keyring, err := credentialprovidersecrets.MakeDockerKeyring(pullSecrets, m.keyring)
	if err != nil {
		return "", err
	}

	var imageService internalapi.ImageManagerService

	if is, err := m.GetImageServiceByPod(image.Pod); err != nil {
		klog.Errorf("GetAllImageServices Failed: %v", err)
	} else {
		klog.V(5).Infof("Got desired image service for image %v, %v", image, is)
		imageService = is
	}

	imgSpec := &runtimeapi.ImageSpec{Image: img}
	creds, withCredentials := keyring.Lookup(repoToPull)
	if !withCredentials {
		klog.V(4).Infof("Pulling image with image service %v", imageService)
		imageRef, err := imageService.PullImage(imgSpec, nil, podSandboxConfig)
		if err != nil {
			klog.Errorf("Pull image %q failed: %v", img, err)
		} else {
			klog.V(5).Infof("Pull image ref for image %q: %v", image, imageRef)
			return imageRef, nil
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

		klog.V(4).Infof("Pulling image with image service %v", imageService)
		imageRef, err := imageService.PullImage(imgSpec, auth, podSandboxConfig)
		if err != nil {
			klog.Errorf("Pull image %q failed: %v", img, err)
		} else {
			klog.V(5).Infof("Pull image ref for image %q: %v", image, imageRef)
			return imageRef, nil
		}
		pullErrs = append(pullErrs, err)
	}

	return "", utilerrors.NewAggregate(pullErrs)
}

// GetImageRef gets the ID of the image which has already been in
// the local storage. It returns ("", nil) if the image isn't in the local storage.
func (m *kubeGenericRuntimeManager) GetImageRef(image kubecontainer.ImageSpec) (string, error) {
	if image.Pod == nil {
		klog.Errorf("Pod is not set in imageSpec: %v", image)
		return "", fmt.Errorf("POD is not set in imageSpec")
	}

	var imageService internalapi.ImageManagerService
	var err error

	if is, err := m.GetImageServiceByPod(image.Pod); err == nil {
		klog.V(5).Infof("Got desired image service for image %v, %v", image, is)
		imageService = is
	} else {
		klog.Errorf("Cannot find desired image service for imageSpec: %v, Error: %v", image, err)
		return "", err
	}

	status, err := imageService.ImageStatus(&runtimeapi.ImageSpec{Image: image.Image})
	if err != nil || status == nil {
		klog.Errorf("ImageStatus for image %q failed: %v", image, err)
	} else {
		klog.V(5).Infof("Got ImageStatus for image %q: %v", image, status)
		return status.Id, nil
	}

	// if not found, return "", nil
	return "", nil
}

// ListImages gets all images currently on the machine.
func (m *kubeGenericRuntimeManager) ListImages() ([]kubecontainer.Image, error) {
	var images []kubecontainer.Image

	imageServices, err := m.runtimeRegistry.GetAllImageServices()
	if err != nil {
		klog.Errorf("GetAllImageServices Failed: %v", err)
		return nil, err
	}

	for _, imageService := range imageServices {
		allImages, err := imageService.ServiceApi.ListImages(nil)
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
// Note that the image_gc_manager uses ImageID in the imageSpec to call the RemoveImages API
//
func (m *kubeGenericRuntimeManager) RemoveImage(image kubecontainer.ImageSpec) error {
	imageServices, err := m.runtimeRegistry.GetAllImageServices()
	if err != nil {
		klog.Errorf("GetAllImageServices  Failed: %v", err)
		return err
	}

	for _, imageService := range imageServices {
		err := imageService.ServiceApi.RemoveImage(&runtimeapi.ImageSpec{Image: image.Image})
		// log the error and continue to next imageServices
		//
		if err != nil {
			klog.Warningf("Remove image %q failed: %v", image.Image, err)
			continue
		}

		return nil
	}

	// return error if reaching here
	return fmt.Errorf("image %q not found", image.Image)
}

// ImageStats returns the statistics of the image.
// Notice that current logic doesn't really work for images which share layers (e.g. docker image),
// this is a known issue, and we'll address this by getting imagefs stats directly from CRI.
// Since the ImageStats is used by Cadvisor and imageGC for disk cleanup, return total size for
// all image services registered at the node.
//
func (m *kubeGenericRuntimeManager) ImageStats() (*kubecontainer.ImageStats, error) {
	imageServices, err := m.runtimeRegistry.GetAllImageServices()
	if err != nil {
		klog.Errorf("GetAllImageServices Failed: %v", err)
		return nil, err
	}

	stats := &kubecontainer.ImageStats{}
	for _, imageService := range imageServices {
		allImages, err := imageService.ServiceApi.ListImages(nil)
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
