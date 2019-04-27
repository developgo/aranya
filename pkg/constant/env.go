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

package constant

import (
	"os"
)

const (
	EnvKeyPodName         = "POD_NAME"
	EnvKeyWatchNamespace  = "WATCH_NAMESPACE"
	EnvKeyAranyaNamespace = "ARANYA_NAMESPACE"
)

func ThisPodName() string {
	return os.Getenv(EnvKeyPodName)
}

func WatchNamespace() string {
	ns := os.Getenv(EnvKeyWatchNamespace)
	if ns == "" {
		return "default"
	}
	return ns
}

func AranyaNamespace() string {
	ns := os.Getenv(EnvKeyAranyaNamespace)
	if ns == "" {
		return WatchNamespace()
	}
	return ns
}
