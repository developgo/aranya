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

package runtime

import (
	"context"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func NewRuntimeBase(ctx context.Context, config *Config, name, version, os, arch, kernelVersion string) Base {
	return Base{
		Config: *config,

		ctx:           ctx,
		name:          name,
		version:       version,
		os:            os,
		arch:          arch,
		kernelVersion: kernelVersion,

		logger: logf.ZapLogger(true).WithName("runtime"),
	}
}

type Base struct {
	Config

	ctx    context.Context
	logger logr.Logger

	name, version, os, arch,
	kernelVersion string
}

func (b *Base) Log() logr.Logger {

	return b.logger
}

func (b *Base) OS() string {
	return b.os
}

func (b *Base) Arch() string {
	return b.arch
}

func (b *Base) KernelVersion() string {
	return b.kernelVersion
}

func (b *Base) Name() string {
	return b.name
}

func (b *Base) Version() string {
	return b.version
}

func (b *Base) ImageActionContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(b.ctx, b.Config.EndPoints.Image.ActionTimeout)
}

func (b *Base) RuntimeActionContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(b.ctx, b.Config.EndPoints.Runtime.ActionTimeout)
}

func (b *Base) ActionContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(b.ctx)
}
