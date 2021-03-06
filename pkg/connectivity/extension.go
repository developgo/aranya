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

package connectivity

import (
	"errors"
	"io/ioutil"
	"path/filepath"
)

func (m *Msg) Err() *Error {
	if m.GetError() == nil {
		return nil
	}

	switch m.GetError().GetKind() {
	case ErrCommon:
		if m.GetError().GetDescription() != "" {
			return m.GetError()
		}

		return nil
	default:
		return m.GetError()
	}
}

func (m *Error) Error() string {
	if m == nil {

	}
	return m.GetKind().String() + "/" + m.GetDescription()
}

func (m *Error) IsCommon() bool {
	return m.isKind(ErrCommon)
}

func (m *Error) IsNotFound() bool {
	return m.isKind(ErrNotFound)
}

func (m *Error) IsAlreadyExists() bool {
	return m.isKind(ErrAlreadyExists)
}

func (m *Error) IsNotSupported() bool {
	return m.isKind(ErrNotSupported)
}

func (m *Error) isKind(kind Error_Kind) bool {
	if m == nil {
		return false
	}

	return m.Kind == kind
}

func (v *MountOptions) Ensure(dir string, dataMap map[string][]byte) (mountPath string, err error) {
	if subPath := v.GetSubPath(); subPath != "" {
		data, ok := dataMap[subPath]
		if !ok {
			return "", errors.New("volume data not found")
		}

		dataFilePath := filepath.Join(dir, subPath)
		if err = ioutil.WriteFile(dataFilePath, data, 0600); err != nil {
			return "", err
		}
		return dataFilePath, nil
	} else {
		for fileName, data := range dataMap {
			dataFilePath := filepath.Join(dir, fileName)
			if err = ioutil.WriteFile(dataFilePath, data, 0600); err != nil {
				return "", err
			}
		}
		return dir, nil
	}
}
