package runtime

import (
	"context"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func NewRuntimeBase(ctx context.Context, name, version string, config *Config) Base {
	return Base{
		Config: *config,

		ctx:     ctx,
		name:    name,
		version: version,
	}
}

type Base struct {
	Config

	ctx     context.Context
	name    string
	version string
}

func (b *Base) Log() logr.Logger {
	return logf.Log
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
