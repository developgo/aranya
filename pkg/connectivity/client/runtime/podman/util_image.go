/*
Copyright 2019 The arhat.dev Authors.

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

package podman

import (
	"context"
	"os"

	imageTypes "github.com/containers/image/types"
	libpodImage "github.com/containers/libpod/libpod/image"
	corev1 "k8s.io/api/core/v1"

	"arhat.dev/aranya/pkg/connectivity"
)

func ensureImages(imageRuntime *libpodImage.Runtime, containers map[string]*connectivity.ContainerSpec, authConfig map[string]*connectivity.AuthConfig) (map[string]*libpodImage.Image, error) {
	imageMap := make(map[string]*libpodImage.Image)
	imageToPull := make([]string, 0)

	for _, ctr := range containers {
		image, err := imageRuntime.NewFromLocal(ctr.Image)
		if err == nil {
			// image exists
			switch ctr.ImagePullPolicy {
			case string(corev1.PullNever), string(corev1.PullIfNotPresent):
				imageMap[ctr.Image] = image
			case string(corev1.PullAlways):
				imageToPull = append(imageToPull, ctr.Image)
			}
		} else {
			// image does not exist
			switch ctr.ImagePullPolicy {
			case string(corev1.PullNever):
				return nil, err
			case string(corev1.PullIfNotPresent), string(corev1.PullAlways):
				imageToPull = append(imageToPull, ctr.Image)
			}
		}
	}

	for _, imageName := range imageToPull {
		config, hasCred := authConfig[imageName]

		var dockerRegistryOptions *libpodImage.DockerRegistryOptions
		if hasCred {
			dockerRegistryOptions = &libpodImage.DockerRegistryOptions{
				DockerRegistryCreds: &imageTypes.DockerAuthConfig{
					Username: config.GetUsername(),
					Password: config.GetPassword(),
				},
				DockerCertPath:              "",
				DockerInsecureSkipTLSVerify: imageTypes.NewOptionalBool(false),
			}
		}

		image, err := imageRuntime.New(context.Background(), imageName, "", "", os.Stderr, dockerRegistryOptions, libpodImage.SigningOptions{}, false, nil)
		if err != nil {
			return nil, err
		}
		imageMap[imageName] = image
	}

	return imageMap, nil
}
