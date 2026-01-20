package service

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"orbit/internal/model"
)

type Manager struct {
	Config    *model.Config
	StatusMap map[string]*model.ProjectStatus
	Mutex     sync.Mutex
}

func NewManager(cfg *model.Config) *Manager {
	statusMap := make(map[string]*model.ProjectStatus)
	for name := range cfg.Projects {
		statusMap[name] = &model.ProjectStatus{Status: "Idle", LastResult: "N/A"}
	}
	return &Manager{
		Config:    cfg,
		StatusMap: statusMap,
	}
}

func (m *Manager) TriggerBuild(projectName string) bool {
	project, exists := m.Config.Projects[projectName]
	if !exists {
		return false
	}

	m.Mutex.Lock()
	if _, ok := m.StatusMap[projectName]; !ok {
		m.StatusMap[projectName] = &model.ProjectStatus{}
	}
	m.StatusMap[projectName].Status = "Building"
	m.StatusMap[projectName].LastRun = time.Now()
	m.Mutex.Unlock()

	go m.runOrbitCycle(projectName, project)
	return true
}

func (m *Manager) runOrbitCycle(name string, p model.Project) {
	m.updateStatus(name, "Cooling Check", "")

	// 1. THERMAL PROTECTION
	for m.GetSystemLoad() > 1.5 {
		log.Printf("[%s] System hot. Holding...", name)
		time.Sleep(30 * time.Second)
	}

	m.updateStatus(name, "Building", "")

	// 2. BACKUP
	binPath := filepath.Join(p.Path, p.BinaryName)
	backupPath := binPath + ".bak"
	m.copyFile(binPath, backupPath)

	// 3. PULL & BUILD
	if err := m.runCmd(p.Path, "git", "pull", "origin", p.Branch); err != nil {
		log.Printf("[%s] Git Pull failed", name)
		m.updateStatus(name, "Idle", "Git Fail")
		return
	}

	if err := m.runShell(p.Path, p.BuildCmd); err != nil {
		log.Printf("[%s] Build failed", name)
		m.updateStatus(name, "Idle", "Build Fail")
		return
	}

	// 4. RESTART
	m.updateStatus(name, "Restarting", "")
	if err := m.runShell(p.Path, p.RestartCmd); err != nil {
		log.Printf("[%s] Restart failed", name)
		m.updateStatus(name, "Idle", "Restart Fail")
		return
	}

	// 5. HEALTH CHECK
	m.updateStatus(name, "Verifying", "")
	time.Sleep(5 * time.Second)

	if m.checkHealth(p.HealthURL) {
		log.Printf("[%s] Success", name)
		m.updateStatus(name, "Idle", "Success")
	} else {
		log.Printf("[%s] Failed. Rolling back...", name)
		m.updateStatus(name, "Rolling Back", "Failed")
		os.Rename(backupPath, binPath)
		m.runShell(p.Path, p.RestartCmd)
		m.updateStatus(name, "Idle", "Rollback")
	}
}

func (m *Manager) updateStatus(name, status, result string) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	s := m.StatusMap[name]
	s.Status = status
	if result != "" {
		s.LastResult = result
	}
}

func (m *Manager) GetSystemLoad() float64 {
	data, _ := os.ReadFile("/proc/loadavg")
	if len(data) == 0 {
		return 0.0
	}
	fields := strings.Fields(string(data))
	load, _ := strconv.ParseFloat(fields[0], 64)
	return load
}

func (m *Manager) runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func (m *Manager) runShell(dir, cmdStr string) error {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir = dir
	return cmd.Run()
}

func (m *Manager) copyFile(src, dst string) {
	input, _ := os.ReadFile(src)
	os.WriteFile(dst, input, 0755)
}

func (m *Manager) checkHealth(url string) bool {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	return true
}
