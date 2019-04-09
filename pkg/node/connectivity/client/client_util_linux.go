package client

import (
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"syscall"
)

func systemInfo() *corev1.NodeSystemInfo {
	bootID, _ := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")
	machineID, _ := ioutil.ReadFile("/etc/machine-id")
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

	return &corev1.NodeSystemInfo{
		MachineID:               string(machineID),
		SystemUUID:              string(systemUUID),
		BootID:                  string(bootID),
		KernelVersion:           kernelVersion,
		OSImage:                 string(osImage),
	}
}
