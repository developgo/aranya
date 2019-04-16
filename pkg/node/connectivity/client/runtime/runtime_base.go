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
