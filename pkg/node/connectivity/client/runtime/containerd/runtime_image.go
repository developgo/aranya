package containerd

import (
	"context"
	"errors"
	"fmt"

	dockerref "github.com/docker/distribution/reference"
	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/credentialprovider"
	credentialprovidersecrets "k8s.io/kubernetes/pkg/credentialprovider/secrets"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/images"
	"k8s.io/kubernetes/pkg/util/parsers"
)

type pullResult struct {
	imageRef string
	err      error
}

// EnsureImageExists pulls the image for the specified pod and container, and returns
// (imageRef, error message, error).
func (r *Runtime) ensureImageExists(pod *corev1.Pod, container *corev1.Container, pullSecrets []corev1.Secret) (string, error) {
	// logPrefix := fmt.Sprintf("%s/%s", pod.Name, container.Image)
	// ref, err := kubecontainer.GenerateContainerRef(pod, container)
	// if err != nil {
	// 	// klog.Errorf("Couldn't make a ref to pod %v, container %v: '%v'", pod.Name, container.Name, err)
	// }

	// If the image contains no tag or digest, a default tag should be applied.
	image, err := applyDefaultImageTag(container.Image)
	if err != nil {
		// msg := fmt.Sprintf("Failed to apply default image tag %q: %v", container.Image, err)
		// m.logIt(ref, corev1.EventTypeWarning, events.FailedToInspectImage, logPrefix, msg, klog.Warning)
		return "", images.ErrInvalidImageName
	}

	spec := kubecontainer.ImageSpec{Image: image}
	imageRef, err := r.GetImageRef(spec)
	if err != nil {
		// msg := fmt.Sprintf("Failed to inspect image %q: %v", container.Image, err)
		// m.logIt(ref, corev1.EventTypeWarning, events.FailedToInspectImage, logPrefix, msg, klog.Warning)
		return "", images.ErrImageInspect
	}

	present := imageRef != ""
	if !shouldPullImage(container, present) {
		if present {
			// msg := fmt.Sprintf("Container image %q already present on machine", container.Image)
			// m.logIt(ref, corev1.EventTypeNormal, events.PulledImage, logPrefix, msg, klog.Info)
			return imageRef, nil
		}
		// msg := fmt.Sprintf("Container image %q is not present with pull policy of Never", container.Image)
		// m.logIt(ref, corev1.EventTypeWarning, events.ErrImageNeverPullPolicy, logPrefix, msg, klog.Warning)
		return "", images.ErrImageNeverPull
	}

	backOffKey := fmt.Sprintf("%s_%s", pod.UID, container.Image)
	if r.imageActionBackOff.IsInBackOffSinceUpdate(backOffKey, r.imageActionBackOff.Clock.Now()) {
		// msg := fmt.Sprintf("Back-off pulling image %q", container.Image)
		// m.logIt(ref, corev1.EventTypeNormal, events.BackOffPullImage, logPrefix, msg, klog.Info)
		return "", images.ErrImagePullBackOff
	}
	// m.logIt(ref, corev1.EventTypeNormal, events.PullingImage, logPrefix, fmt.Sprintf("pulling image %q", container.Image), klog.Info)
	pullChan := make(chan pullResult)
	r.pullImage(spec, pullSecrets, pullChan)
	imagePullResult := <-pullChan
	if imagePullResult.err != nil {
		// m.logIt(ref, corev1.EventTypeWarning, events.FailedToPullImage, logPrefix, fmt.Sprintf("Failed to pull image %q: %v", container.Image, imagePullResult.err), klog.Warning)
		r.imageActionBackOff.Next(backOffKey, r.imageActionBackOff.Clock.Now())
		if imagePullResult.err == images.ErrRegistryUnavailable {
			// msg := fmt.Sprintf("image pull failed for %s because the registry is unavailable.", container.Image)
			return "", imagePullResult.err
		}

		return "", images.ErrImagePull
	}
	// m.logIt(ref, corev1.EventTypeNormal, events.PulledImage, logPrefix, fmt.Sprintf("Successfully pulled image %q", container.Image), klog.Info)
	r.imageActionBackOff.GC()

	return imagePullResult.imageRef, nil
}

// GetImageRef gets the ID of the image which has already been in
// the local storage. It returns ("", nil) if the image isn't in the local storage.
func (r *Runtime) GetImageRef(image kubecontainer.ImageSpec) (string, error) {
	status, err := r.ImageStatus(&criRuntime.ImageSpec{Image: image.Image})
	if err != nil {
		// klog.Errorf("ImageStatus for image %q failed: %v", image, err)
		return "", err
	}
	if status == nil {
		return "", nil
	}
	return status.Id, nil
}

// applyDefaultImageTag parses a docker image string, if it doesn't contain any tag or digest,
// a default tag will be applied.
func applyDefaultImageTag(image string) (string, error) {
	named, err := dockerref.ParseNormalizedNamed(image)
	if err != nil {
		return "", fmt.Errorf("couldn't parse image reference %q: %v", image, err)
	}
	_, isTagged := named.(dockerref.Tagged)
	_, isDigested := named.(dockerref.Digested)
	if !isTagged && !isDigested {
		// we just concatenate the image name with the default tag here instead
		// of using dockerref.WithTag(named, ...) because that would cause the
		// image to be fully qualified as docker.io/$name if it's a short name
		// (e.g. just busybox). We don't want that to happen to keep the CRI
		// agnostic wrt image names and default hostnames.
		image = image + ":" + parsers.DefaultImageTag
	}
	return image, nil
}

// shouldPullImage returns whether we should pull an image according to
// the presence and pull policy of the image.
func shouldPullImage(container *corev1.Container, imagePresent bool) bool {
	if container.ImagePullPolicy == corev1.PullNever {
		return false
	}

	if container.ImagePullPolicy == corev1.PullAlways ||
		(container.ImagePullPolicy == corev1.PullIfNotPresent && (!imagePresent)) {
		return true
	}

	return false
}

// ListImages lists available images.
func (r *Runtime) ListImages(filter *criRuntime.ImageFilter) ([]*criRuntime.Image, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.imageActionTimeout)
	defer cancel()

	resp, err := r.imageSvcClient.ListImages(ctx, &criRuntime.ListImagesRequest{
		Filter: filter,
	})

	if err != nil {
		// klog.Errorf("ListImages with filter %+v from image service failed: %v", filter, err)
		return nil, err
	}

	return resp.Images, nil
}

// ImageStatus returns the status of the image.
func (r *Runtime) ImageStatus(image *criRuntime.ImageSpec) (*criRuntime.Image, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.imageActionTimeout)
	defer cancel()

	resp, err := r.imageSvcClient.ImageStatus(ctx, &criRuntime.ImageStatusRequest{
		Image: image,
	})
	if err != nil {
		// klog.Errorf("ImageStatus %q from image service failed: %v", image.Image, err)
		return nil, err
	}

	if resp.Image != nil {
		if resp.Image.Id == "" || resp.Image.Size_ == 0 {
			errorMessage := fmt.Sprintf("Id or size of image %q is not set", image.Image)
			// klog.Errorf("ImageStatus failed: %s", errorMessage)
			return nil, errors.New(errorMessage)
		}
	}

	return resp.Image, nil
}

func (r *Runtime) pullImage(spec kubecontainer.ImageSpec, pullSecrets []corev1.Secret, pullChan chan<- pullResult) {
	go func() {
		imageRef, err := r.PullImage(spec, pullSecrets)
		pullChan <- pullResult{
			imageRef: imageRef,
			err:      err,
		}
	}()
}

// PullImage pulls an image from the network to local storage using the supplied
// secrets if necessary.
func (r *Runtime) PullImage(image kubecontainer.ImageSpec, pullSecrets []corev1.Secret) (string, error) {
	img := image.Image
	repoToPull, _, _, err := parsers.ParseImageName(img)
	if err != nil {
		return "", err
	}

	keyring, err := credentialprovidersecrets.MakeDockerKeyring(pullSecrets, credentialprovider.NewDockerKeyring())
	if err != nil {
		return "", err
	}

	imgSpec := &criRuntime.ImageSpec{Image: img}
	creds, withCredentials := keyring.Lookup(repoToPull)
	if !withCredentials {
		// klog.V(3).Infof("Pulling image %q without credentials", img)

		imageRef, err := r.remotePullImage(imgSpec, nil)
		if err != nil {
			// klog.Errorf("Pull image %q failed: %v", img, err)
			return "", err
		}

		return imageRef, nil
	}

	var pullErrs []error
	for _, currentCreds := range creds {
		authConfig := credentialprovider.LazyProvide(currentCreds)
		auth := &criRuntime.AuthConfig{
			Username:      authConfig.Username,
			Password:      authConfig.Password,
			Auth:          authConfig.Auth,
			ServerAddress: authConfig.ServerAddress,
			IdentityToken: authConfig.IdentityToken,
			RegistryToken: authConfig.RegistryToken,
		}

		imageRef, err := r.remotePullImage(imgSpec, auth)
		// If there was no error, return success
		if err == nil {
			return imageRef, nil
		}

		pullErrs = append(pullErrs, err)
	}

	return "", utilerrors.NewAggregate(pullErrs)
}

// PullImage pulls an image with authentication config.
func (r *Runtime) remotePullImage(image *criRuntime.ImageSpec, auth *criRuntime.AuthConfig) (string, error) {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	resp, err := r.imageSvcClient.PullImage(ctx, &criRuntime.PullImageRequest{
		Image: image,
		Auth:  auth,
	})
	if err != nil {
		klog.Errorf("PullImage %q from image service failed: %v", image.Image, err)
		return "", err
	}

	if resp.ImageRef == "" {
		errorMessage := fmt.Sprintf("imageRef of image %q is not set", image.Image)
		klog.Errorf("PullImage failed: %s", errorMessage)
		return "", errors.New(errorMessage)
	}

	return resp.ImageRef, nil
}

// RemoveImage removes the image.
func (r *Runtime) RemoveImage(image *criRuntime.ImageSpec) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.imageActionTimeout)
	defer cancel()

	_, err := r.imageSvcClient.RemoveImage(ctx, &criRuntime.RemoveImageRequest{
		Image: image,
	})
	if err != nil {
		// klog.Errorf("RemoveImage %q from image service failed: %v", image.Image, err)
		return err
	}

	return nil
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (r *Runtime) ImageFsInfo() ([]*criRuntime.FilesystemUsage, error) {
	// Do not set timeout, because `ImageFsInfo` takes time.
	// TODO(random-liu): Should we assume runtime should cache the result, and set timeout here?
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	resp, err := r.imageSvcClient.ImageFsInfo(ctx, &criRuntime.ImageFsInfoRequest{})
	if err != nil {
		// klog.Errorf("ImageFsInfo from image service failed: %v", err)
		return nil, err
	}
	return resp.GetImageFilesystems(), nil
}
