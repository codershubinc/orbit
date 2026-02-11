package codershubinc

type MachineInfo struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Uptime       string `json:"uptime"`

	CPUModel        string `json:"cpu_model"`
	CPUCores        int    `json:"cpu_cores"`
	CPUUsagePercent int    `json:"cpu_usage_percent"`

	GPUModel string `json:"gpu_model"`
	GPUUses  int    `json:"gpu_uses_percent"`

	// TotalMemory uint64 `json:"total_memory"`
	// UsedMemory  uint64 `json:"used_memory"`
	// FreeMemory  uint64 `json:"free_memory"`

	// DiskTotal uint64 `json:"disk_total"`
	// DiskUsed  uint64 `json:"disk_used"`
	// DiskFree  uint64 `json:"disk_free"`

	// NetworkSent     uint64 `json:"network_sent"`
	// NetworkReceived uint64 `json:"network_received"`
}

func GatherMachineInfo() (m MachineInfo, err error) {

	h, err := hostInfo()
	if err != nil {
		return m, err
	}
	m.Hostname = h.Hostname
	m.OS = h.OS
	m.Architecture = h.Architecture
	m.Uptime = h.Uptime

	c, err := cpuInfo()
	if err != nil {
		return m, err
	}
	m.CPUModel = c.Model
	m.CPUCores = c.Cores
	m.CPUUsagePercent = c.UsagePercent

	g, err := gpuInfo()
	if err == nil {
		m.GPUModel = g.GPUModel
		m.GPUUses = g.GPUUses
	}

	return m, nil

}
