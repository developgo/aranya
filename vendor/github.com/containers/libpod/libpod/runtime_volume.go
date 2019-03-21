package libpod

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strings"
)

// Contains the public Runtime API for volumes

// A VolumeCreateOption is a functional option which alters the Volume created by
// NewVolume
type VolumeCreateOption func(*Volume) error

// VolumeFilter is a function to determine whether a volume is included in command
// output. Volumes to be outputted are tested using the function. a true return will
// include the volume, a false return will exclude it.
type VolumeFilter func(*Volume) bool

// RemoveVolume removes a volumes
func (r *Runtime) RemoveVolume(ctx context.Context, v *Volume, force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	if !v.valid {
		if ok, _ := r.state.HasVolume(v.Name()); !ok {
			// Volume probably already removed
			// Or was never in the runtime to begin with
			return nil
		}
	}

	return r.removeVolume(ctx, v, force)
}

// RemoveVolumes removes a slice of volumes or all with a force bool
func (r *Runtime) RemoveVolumes(ctx context.Context, volumes []string, all, force bool) ([]string, error) {
	var (
		vols        []*Volume
		err         error
		deletedVols []string
	)
	if all {
		vols, err = r.Volumes()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get all volumes")
		}
	} else {
		for _, i := range volumes {
			vol, err := r.GetVolume(i)
			if err != nil {
				return nil, err
			}
			vols = append(vols, vol)
		}
	}

	for _, vol := range vols {
		if err := r.RemoveVolume(ctx, vol, force); err != nil {
			return deletedVols, err
		}
		logrus.Debugf("removed volume %s", vol.Name())
		deletedVols = append(deletedVols, vol.Name())
	}
	return deletedVols, nil
}

// GetVolume retrieves a volume by its name
func (r *Runtime) GetVolume(name string) (*Volume, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	vol, err := r.state.Volume(name)
	if err == nil {
		return vol, err
	}

	vols, err := r.GetAllVolumes()
	if err != nil {
		return nil, err
	}
	for _, v := range vols {
		if strings.HasPrefix(v.Name(), name) {
			return v, nil
		}
	}
	return nil, errors.Errorf("unable to find volume %s", name)
}

// HasVolume checks to see if a volume with the given name exists
func (r *Runtime) HasVolume(name string) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return false, ErrRuntimeStopped
	}

	return r.state.HasVolume(name)
}

// Volumes retrieves all volumes
// Filters can be provided which will determine which volumes are included in the
// output. Multiple filters are handled by ANDing their output, so only volumes
// matching all filters are returned
func (r *Runtime) Volumes(filters ...VolumeFilter) ([]*Volume, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	vols, err := r.state.AllVolumes()
	if err != nil {
		return nil, err
	}

	volsFiltered := make([]*Volume, 0, len(vols))
	for _, vol := range vols {
		include := true
		for _, filter := range filters {
			include = include && filter(vol)
		}

		if include {
			volsFiltered = append(volsFiltered, vol)
		}
	}

	return volsFiltered, nil
}

// GetAllVolumes retrieves all the volumes
func (r *Runtime) GetAllVolumes() ([]*Volume, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.AllVolumes()
}

// PruneVolumes removes unused volumes from the system
func (r *Runtime) PruneVolumes(ctx context.Context) ([]string, []error) {
	var (
		prunedIDs   []string
		pruneErrors []error
	)
	vols, err := r.GetAllVolumes()
	if err != nil {
		pruneErrors = append(pruneErrors, err)
		return nil, pruneErrors
	}

	for _, vol := range vols {
		if err := r.RemoveVolume(ctx, vol, false); err != nil {
			if errors.Cause(err) != ErrVolumeBeingUsed && errors.Cause(err) != ErrVolumeRemoved {
				pruneErrors = append(pruneErrors, err)
			}
			continue
		}
		prunedIDs = append(prunedIDs, vol.Name())
	}
	return prunedIDs, pruneErrors
}
