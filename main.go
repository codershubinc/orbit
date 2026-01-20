package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- CONFIGURATION ---
type Config struct {
	Port     int                `json:"port"`
	Secret   string             `json:"webhook_secret"`
	Projects map[string]Project `json:"projects"`
}

type Project struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	BuildCmd   string `json:"build_cmd"`
	RestartCmd string `json:"restart_cmd"`
	BinaryName string `json:"binary_name"`
	HealthURL  string `json:"health_url"`
}

// --- STATE MANAGEMENT (For the Dashboard) ---
type ProjectStatus struct {
	LastRun    time.Time
	Status     string // "Idle", "Building", "Success", "Failed"
	LastResult string // "Success" or "Rollback"
}

var (
	config    Config
	statusMap = make(map[string]*ProjectStatus)
	mutex     sync.Mutex // Protects map from concurrent writes
)

func main() {
	// 0. Print Banner
	fmt.Println(`
   ____  ____  ____  __  ______
  / __ \/ __ \/ __ \/  |/  /_  __/
 / /_/ / /_/ / /_/ / /|_/ / / /   
 \____/_/ |_/_____/_/  /_/ /_/    
      v1.1 • Dashboard Active
	`)

	// 1. Load Config
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("❌ [Orbit] Could not read config.json: %v", err)
	}
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Fatalf("❌ [Orbit] Could not parse config.json: %v", err)
	}

	// Initialize status map
	for name := range config.Projects {
		statusMap[name] = &ProjectStatus{Status: "Idle", LastResult: "N/A"}
	}

	// 2. Start Server
	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/", handleDashboard)    // <--- New Home Route
	http.HandleFunc("/config", handleConfig) // <--- Config Editor

	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("🔭 [Orbit] Dashboard live at http://localhost:%d", config.Port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// --- DASHBOARD HANDLER ---
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Prepare Data for Template
	load := getSystemLoad()

	type ProjectView struct {
		Status, ResultClass, StateClass, LastResult, TimeAgo string
	}

	data := struct {
		Load       float64
		Overloaded bool
		Projects   map[string]ProjectView
	}{
		Load:       load,
		Overloaded: load > 1.5,
		Projects:   make(map[string]ProjectView),
	}

	mutex.Lock()
	for name, s := range statusMap {
		stateClass := strings.ToLower(s.Status)
		resultClass := "idle"
		if s.LastResult == "Success" {
			resultClass = "success"
		}
		if s.LastResult == "Rollback" {
			resultClass = "failed"
		}

		timeStr := "Never"
		if !s.LastRun.IsZero() {
			timeStr = s.LastRun.Format("15:04:05")
		}

		data.Projects[name] = ProjectView{
			Status:      s.Status,
			StateClass:  stateClass,
			LastResult:  s.LastResult,
			ResultClass: resultClass,
			TimeAgo:     timeStr,
		}
	}
	mutex.Unlock()

	tmpl, err := template.ParseFiles("templates/dashboard.html")
	if err != nil {
		http.Error(w, "Could not load dashboard template", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}
	tmpl.Execute(w, data)
}

// --- WEBHOOK HANDLER ---
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("secret") != config.Secret {
		http.Error(w, "Forbidden", 403)
		return
	}
	// ... func continues ...

	projectName := r.URL.Query().Get("project")
	project, exists := config.Projects[projectName]
	if !exists {
		http.Error(w, "Project not found", 404)
		return
	}

	// Update Status to "Building"
	mutex.Lock()
	if _, ok := statusMap[projectName]; !ok {
		statusMap[projectName] = &ProjectStatus{}
	}
	statusMap[projectName].Status = "Building"
	statusMap[projectName].LastRun = time.Now()
	mutex.Unlock()

	go runOrbitCycle(projectName, project)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Orbit initiated"))
}

// --- DEPLOYMENT CYCLE ---
func runOrbitCycle(name string, p Project) {
	updateStatus(name, "Cooling Check", "")

	// 1. THERMAL PROTECTION
	for getSystemLoad() > 1.5 {
		log.Printf("[%s] System hot. Holding...", name)
		time.Sleep(30 * time.Second)
	}

	updateStatus(name, "Building", "")

	// 2. BACKUP
	binPath := filepath.Join(p.Path, p.BinaryName)
	backupPath := binPath + ".bak"
	copyFile(binPath, backupPath)

	// 3. PULL & BUILD
	if err := runCmd(p.Path, "git", "pull", "origin", p.Branch); err != nil {
		log.Printf("[%s] Git Pull failed", name)
		updateStatus(name, "Idle", "Git Fail")
		return
	}

	if err := runShell(p.Path, p.BuildCmd); err != nil {
		log.Printf("[%s] Build failed", name)
		updateStatus(name, "Idle", "Build Fail")
		return
	}

	// 4. RESTART
	updateStatus(name, "Restarting", "")
	if err := runShell(p.Path, p.RestartCmd); err != nil {
		log.Printf("[%s] Restart failed", name)
		updateStatus(name, "Idle", "Restart Fail")
		return
	}

	// 5. HEALTH CHECK
	updateStatus(name, "Verifying", "")
	time.Sleep(5 * time.Second)

	if checkHealth(p.HealthURL) {
		log.Printf("[%s] Success", name)
		updateStatus(name, "Idle", "Success")
	} else {
		log.Printf("[%s] Failed. Rolling back...", name)
		updateStatus(name, "Rolling Back", "Failed")
		os.Rename(backupPath, binPath)
		runShell(p.Path, p.RestartCmd)
		updateStatus(name, "Idle", "Rollback")
	}
}

// --- HELPERS ---
func updateStatus(name, status, result string) {
	mutex.Lock()
	defer mutex.Unlock()
	s := statusMap[name]
	s.Status = status
	if result != "" {
		s.LastResult = result
	}
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func runShell(dir, cmdStr string) error {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir = dir
	return cmd.Run()
}

func copyFile(src, dst string) {
	input, _ := os.ReadFile(src)
	os.WriteFile(dst, input, 0755)
}

func checkHealth(url string) bool {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	return true
}

func getSystemLoad() float64 {
	data, _ := os.ReadFile("/proc/loadavg")
	if len(data) == 0 {
		return 0.0
	}
	fields := strings.Fields(string(data))
	load, _ := strconv.ParseFloat(fields[0], 64)
	return load
}

// --- CONFIG HANDLER ---
func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// Update Global
		portStr := r.FormValue("global_port")
		port, err := strconv.Atoi(portStr)
		if err == nil {
			config.Port = port
		}
		config.Secret = r.FormValue("global_secret")

		// Update Projects
		for name, proj := range config.Projects {
			proj.Path = r.FormValue("proj_" + name + "_path")
			proj.Branch = r.FormValue("proj_" + name + "_branch")
			proj.BuildCmd = r.FormValue("proj_" + name + "_build")
			proj.RestartCmd = r.FormValue("proj_" + name + "_restart")
			proj.BinaryName = r.FormValue("proj_" + name + "_binary")
			proj.HealthURL = r.FormValue("proj_" + name + "_health")
			config.Projects[name] = proj
		}

		// Serialize
		data, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
			return
		}

		// Save
		if err := os.WriteFile("config.json", data, 0644); err != nil {
			http.Error(w, "Failed to write config file", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/config", http.StatusSeeOther)
		return
	}

	// GET Request - Render Template
	tmpl, err := template.ParseFiles("templates/config.html")
	if err != nil {
		http.Error(w, "Could not load config template", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}
	tmpl.Execute(w, config)
}
