package docker

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

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	dockerType "github.com/docker/docker/api/types"
	dockerFilter "github.com/docker/docker/api/types/filters"
	dockerMessage "github.com/docker/docker/pkg/jsonmessage"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtimeutil"
)

func (r *dockerRuntime) ensureImages(containers map[string]*connectivity.ContainerSpec, authConfig map[string]*connectivity.AuthConfig) (map[string]*dockerType.ImageSummary, *connectivity.Error) {
	var (
		imageMap    = make(map[string]*dockerType.ImageSummary)
		imageToPull = make([]string, 0)
	)

	pullCtx, cancelPull := r.ImageActionContext()
	defer cancelPull()

	for _, ctr := range containers {
		if ctr.GetImagePullPolicy() == connectivity.ImagePullAlways {
			imageToPull = append(imageToPull, ctr.GetImage())
			continue
		}

		image, err := r.getImage(pullCtx, ctr.Image)
		if err == nil {
			// image exists
			switch ctr.GetImagePullPolicy() {
			case connectivity.ImagePullNever, connectivity.ImagePullIfNotPresent:
				imageMap[ctr.GetImage()] = image
			}
		} else {
			// image does not exist
			switch ctr.GetImagePullPolicy() {
			case connectivity.ImagePullNever:
				return nil, connectivity.NewCommonError(err.Error())
			case connectivity.ImagePullIfNotPresent:
				imageToPull = append(imageToPull, ctr.GetImage())
			}
		}
	}

	for _, imageName := range imageToPull {
		authStr := ""
		if authConfig != nil {
			config, hasCred := authConfig[imageName]
			if hasCred {
				authCfg := dockerType.AuthConfig{
					Username:      config.GetUsername(),
					Password:      config.GetPassword(),
					ServerAddress: config.GetServerAddress(),
					IdentityToken: config.GetIdentityToken(),
					RegistryToken: config.GetRegistryToken(),
				}
				encodedJSON, err := json.Marshal(authCfg)
				if err != nil {
					panic(err)
				}
				authStr = base64.URLEncoding.EncodeToString(encodedJSON)
			}
		}

		out, err := r.imageClient.ImagePull(pullCtx, imageName, dockerType.ImagePullOptions{
			RegistryAuth: authStr,
		})
		if err != nil {
			return nil, connectivity.NewCommonError(err.Error())
		}
		err = func() error {
			defer func() { _ = out.Close() }()
			decoder := json.NewDecoder(out)
			for {
				var msg dockerMessage.JSONMessage
				err := decoder.Decode(&msg)
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if msg.Error != nil {
					return msg.Error
				}
			}
			return nil
		}()
		if err != nil {
			return nil, connectivity.NewCommonError(err.Error())
		}

		image, err := r.getImage(pullCtx, imageName)
		if err != nil {
			return nil, connectivity.NewCommonError(err.Error())
		}
		imageMap[imageName] = image
	}

	return imageMap, nil
}

func (r *dockerRuntime) getImage(ctx context.Context, imageName string) (*dockerType.ImageSummary, *connectivity.Error) {
	imageList, err := r.imageClient.ImageList(ctx, dockerType.ImageListOptions{
		Filters: dockerFilter.NewArgs(dockerFilter.Arg("reference", imageName)),
	})
	if err != nil {
		return nil, connectivity.NewCommonError(err.Error())
	}

	if len(imageList) == 0 {
		return nil, runtimeutil.ErrNotFound
	}

	return &imageList[0], nil
}
