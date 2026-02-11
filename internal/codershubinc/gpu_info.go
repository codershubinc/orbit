package codershubinc

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

type GPUInfo struct {
	GPUModel string `json:"gpu_model"`
	GPUUses  int    `json:"gpu_uses_percent"`
}

func gpuInfo() (g GPUInfo, err error) {
	// We use the nvidia-smi tool, which comes installed with NVIDIA drivers.
	// We query specifically for the name and current utilization.
	// --format=csv,noheader,nounits gives us clean data like: "Tesla T4, 0"
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,utilization.gpu", "--format=csv,noheader,nounits")

	var out bytes.Buffer
	cmd.Stdout = &out

	// If this returns an error, it likely means nvidia-smi is missing
	// (no NVIDIA GPU or drivers not installed).
	if err := cmd.Run(); err != nil {
		// You might want to return a specific "No GPU found" error here
		return g, err
	}

	// Parse the output (e.g., "NVIDIA GeForce RTX 3080, 14")
	output := strings.TrimSpace(out.String())
	parts := strings.Split(output, ",")

	if len(parts) >= 2 {
		g.GPUModel = parts[0]

		// Parse utilization (string to int)
		usageVal, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err == nil {
			g.GPUUses = usageVal
		}
	}

	return g, nil
}
