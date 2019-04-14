package cri

import (
	"context"
	"errors"
	"fmt"

	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func (r *Runtime) remoteListImages(filter *criRuntime.ImageFilter) ([]*criRuntime.Image, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.imageActionTimeout)
	defer cancel()

	resp, err := r.imageSvcClient.ListImages(ctx, &criRuntime.ListImagesRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, err
	}

	return resp.Images, nil
}

func (r *Runtime) remoteImageStatus(image *criRuntime.ImageSpec) (*criRuntime.Image, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.imageActionTimeout)
	defer cancel()

	resp, err := r.imageSvcClient.ImageStatus(ctx, &criRuntime.ImageStatusRequest{
		Image: image,
	})
	if err != nil {
		return nil, err
	}

	if resp.Image != nil {
		if resp.Image.Id == "" || resp.Image.Size_ == 0 {
			errorMessage := fmt.Sprintf("Id or size of image %q is not set", image.Image)
			return nil, errors.New(errorMessage)
		}
	}

	return resp.Image, nil
}

func (r *Runtime) remotePullImage(image *criRuntime.ImageSpec, auth *criRuntime.AuthConfig) (string, error) {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	resp, err := r.imageSvcClient.PullImage(ctx, &criRuntime.PullImageRequest{
		Image: image,
		Auth:  auth,
	})
	if err != nil {
		return "", err
	}

	if resp.ImageRef == "" {
		errorMessage := fmt.Sprintf("imageRef of image %q is not set", image.Image)
		return "", errors.New(errorMessage)
	}

	return resp.ImageRef, nil
}

func (r *Runtime) remoteRemoveImage(image *criRuntime.ImageSpec) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.imageActionTimeout)
	defer cancel()

	_, err := r.imageSvcClient.RemoveImage(ctx, &criRuntime.RemoveImageRequest{
		Image: image,
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *Runtime) remoteImageFsInfo() ([]*criRuntime.FilesystemUsage, error) {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	resp, err := r.imageSvcClient.ImageFsInfo(ctx, &criRuntime.ImageFsInfoRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetImageFilesystems(), nil
}
