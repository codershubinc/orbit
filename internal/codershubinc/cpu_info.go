package codershubinc

import cpu "github.com/shirou/gopsutil/v3/cpu"

type CPUInfo struct {
	Model        string `json:"model"`
	Cores        int    `json:"cores"`
	UsagePercent int    `json:"usage_percent"`
}

func cpuInfo() (c CPUInfo, err error) {

	info, err := cpu.Info()
	if err != nil {
		return c, err
	}

	if len(info) > 0 {
		c.Model = info[0].Model
		c.Cores = int(info[0].Cores)
	}

	usage, err := cpu.Percent(0, false)
	if err != nil {
		return c, err
	}
	if len(usage) > 0 {
		c.UsagePercent = int(usage[0])
	}
	return c, nil

}
