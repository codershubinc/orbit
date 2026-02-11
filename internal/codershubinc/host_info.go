package codershubinc

import (
	"os"
	"runtime"
)

type HostInfo struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Uptime       string `json:"uptime"`
}

func hostInfo() (h HostInfo, err error) {

	// get hostname

	h.Hostname, err = os.Hostname()
	if err != nil {
		return h, err
	}

	h.OS = runtime.GOOS
	h.Architecture = runtime.GOARCH

	//get uptime
	uptime := os.Getenv("UPTIME")
	if uptime == "" {
		uptime = "Unknown"
	}
	h.Uptime = uptime
	return h, nil
}
