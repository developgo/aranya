// +build linux

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

package client

import (
	"io/ioutil"
	"strings"
	"syscall"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func setSystemInfo(info *connectivity.NodeSystemInfo) *connectivity.NodeSystemInfo {
	bootID, _ := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")
	osImage, _ := ioutil.ReadFile("/etc/os-release")
	systemUUID, _ := ioutil.ReadFile("/sys/devices/virtual/dmi/id/product_uuid")

	var uname syscall.Utsname
	_ = syscall.Uname(&uname)
	var buf [65]byte
	for i, b := range uname.Release {
		buf[i] = byte(b)
	}
	kernelVersion := string(buf[:])
	if i := strings.Index(kernelVersion, "\x00"); i != -1 {
		kernelVersion = kernelVersion[:i]
	}

	info.SystemUuid = string(systemUUID)
	info.BootId = string(bootID)
	info.KernelVersion = kernelVersion
	info.OsImage = string(osImage)

	return info
}
