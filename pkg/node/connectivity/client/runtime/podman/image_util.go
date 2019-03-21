package podman

import (
	"context"
	"os"

	imageTypes "github.com/containers/image/types"
	libpodImage "github.com/containers/libpod/libpod/image"
	corev1 "k8s.io/api/core/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func ensureImages(imageRuntime *libpodImage.Runtime, pod *corev1.PodSpec, authConfig map[string]*criRuntime.AuthConfig) (map[string]*libpodImage.Image, error) {
	imageMap := make(map[string]*libpodImage.Image)
	imageToPull := make([]string, 0)

	for _, ctr := range pod.Containers {
		image, err := imageRuntime.NewFromLocal(ctr.Name)
		if err == nil {
			// image exists
			switch ctr.ImagePullPolicy {
			case corev1.PullNever, corev1.PullIfNotPresent:
				imageMap[ctr.Image] = image
			case corev1.PullAlways:
				imageToPull = append(imageToPull, ctr.Image)
			}
		} else {
			// image does not exist
			switch ctr.ImagePullPolicy {
			case corev1.PullNever:
				return nil, err
			case corev1.PullIfNotPresent, corev1.PullAlways:
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
