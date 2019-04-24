// +build linux

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
