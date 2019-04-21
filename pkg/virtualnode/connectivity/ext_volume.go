package connectivity

import (
	"errors"
	"io/ioutil"
	"path/filepath"
)

func (v *VolumeMount) Ensure(dir string, dataMap map[string][]byte) (mountPath string, err error) {
	if v.GetVolumeSubPath() != "" {
		data, ok := dataMap[v.GetVolumeSubPath()]
		if !ok {
			return "", errors.New("volume data not found")
		}
		dataFilePath := filepath.Join(dir, v.GetVolumeSubPath())
		if err = ioutil.WriteFile(dataFilePath, data, 0600); err != nil {
			return "", err
		}
		return dataFilePath, nil
	} else {
		for fileName, data := range dataMap {
			dataFilePath := filepath.Join(dir, fileName)
			if err := ioutil.WriteFile(dataFilePath, data, 0600); err != nil {
				return "", err
			}
		}
		return dir, nil
	}
}
