package handler

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"orbit/internal/config"
	"orbit/internal/model"
	"orbit/internal/service"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Handler struct {
	Manager *service.Manager
}

func NewHandler(m *service.Manager) *Handler {
	return &Handler{Manager: m}
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	data, err := templatesFS.ReadFile("templates/dashboard.html")
	if err != nil {
		http.Error(w, "Could not load dashboard", http.StatusInternalServerError)
		log.Printf("File read error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("secret") != h.Manager.Config.Secret {
		http.Error(w, "Forbidden", 403)
		return
	}

	projectName := r.URL.Query().Get("project")
	if success := h.Manager.TriggerBuild(projectName); !success {
		http.Error(w, "Project not found", 404)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Orbit initiated"))
}

func (h *Handler) Build(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.FormValue("project")
	if projectName == "" {
		http.Error(w, "Project name required", http.StatusBadRequest)
		return
	}

	if success := h.Manager.TriggerBuild(projectName); !success {
		http.Error(w, "Project not found", 404)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// Update Global
		portStr := r.FormValue("global_port")
		port, err := strconv.Atoi(portStr)
		if err == nil {
			h.Manager.Config.Port = port
		}
		h.Manager.Config.Secret = r.FormValue("global_secret")

		// Update Projects
		for name, proj := range h.Manager.Config.Projects {
			proj.Path = r.FormValue("proj_" + name + "_path")
			proj.Branch = r.FormValue("proj_" + name + "_branch")
			proj.BuildCmd = r.FormValue("proj_" + name + "_build")
			proj.RestartCmd = r.FormValue("proj_" + name + "_restart")
			proj.BinaryName = r.FormValue("proj_" + name + "_binary")
			proj.HealthURL = r.FormValue("proj_" + name + "_health")
			h.Manager.Config.Projects[name] = proj
		}
		// Add New Project if Name is Provided
		newProjName := r.FormValue("new_project_name")
		if newProjName != "" {
			if h.Manager.Config.Projects == nil {
				h.Manager.Config.Projects = make(map[string]model.Project)
			}
			h.Manager.Config.Projects[newProjName] = model.Project{
				Path:       r.FormValue("new_project_path"),
				Branch:     r.FormValue("new_project_branch"),
				BuildCmd:   r.FormValue("new_project_build"),
				RestartCmd: r.FormValue("new_project_restart"),
				BinaryName: r.FormValue("new_project_binary"),
				HealthURL:  r.FormValue("new_project_health"),
			}
		}

		// Save Logic
		// Ideally pass a path, for now hardcoding or using global config path logic if we had one
		if err := config.Save("config.json", h.Manager.Config); err != nil {
			http.Error(w, "Failed to write config file", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/config", http.StatusSeeOther)
		return
	}

	// GET Request - Serve HTML directly (API-driven)
	data, err := templatesFS.ReadFile("templates/config.html")
	if err != nil {
		http.Error(w, "Could not load config page", http.StatusInternalServerError)
		log.Printf("File read error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// ConfigJSONHandler handles the raw JSON editor if we still want it,
// but the requirement said "text boxes", so handleConfig above covers it.
func (h *Handler) ConfigJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		jsonContent := r.FormValue("config_json")
		var tempConfig model.Config
		if err := json.Unmarshal([]byte(jsonContent), &tempConfig); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		if err := config.Save("config.json", &tempConfig); err != nil {
			http.Error(w, "Failed to write config file", http.StatusInternalServerError)
			return
		}
		*h.Manager.Config = tempConfig
		http.Redirect(w, r, "/config-json", http.StatusSeeOther)
		return
	}
	// ... implementation omitted as we moved to form based ...
}

// API Handlers

func (h *Handler) GetConfigAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(h.Manager.Config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) UpdateGlobalConfigAPI(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Port   int    `json:"port"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.Manager.Config.Port = payload.Port
	h.Manager.Config.Secret = payload.Secret

	if err := config.Save("config.json", h.Manager.Config); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) CreateProjectAPI(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name          string `json:"name"`
		model.Project        // Embed project fields
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.Name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	if h.Manager.Config.Projects == nil {
		h.Manager.Config.Projects = make(map[string]model.Project)
	}

	if _, exists := h.Manager.Config.Projects[payload.Name]; exists {
		http.Error(w, "Project already exists", http.StatusConflict)
		return
	}

	h.Manager.Config.Projects[payload.Name] = payload.Project

	if err := config.Save("config.json", h.Manager.Config); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) UpdateProjectAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	var payload model.Project
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, exists := h.Manager.Config.Projects[name]; !exists {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	h.Manager.Config.Projects[name] = payload

	if err := config.Save("config.json", h.Manager.Config); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteProjectAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	if _, exists := h.Manager.Config.Projects[name]; !exists {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	delete(h.Manager.Config.Projects, name)

	if err := config.Save("config.json", h.Manager.Config); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetDashboardAPI(w http.ResponseWriter, r *http.Request) {
	load := h.Manager.GetSystemLoad()

	type ProjectView struct {
		Name        string `json:"name"`
		Status      string `json:"status"`
		ResultClass string `json:"result_class"`
		StateClass  string `json:"state_class"`
		LastResult  string `json:"last_result"`
		TimeAgo     string `json:"time_ago"`
	}

	data := struct {
		Load       float64       `json:"load"`
		Overloaded bool          `json:"overloaded"`
		Projects   []ProjectView `json:"projects"`
	}{
		Load:       load,
		Overloaded: load > 1.5,
		Projects:   make([]ProjectView, 0),
	}

	h.Manager.Mutex.Lock()
	for name, s := range h.Manager.StatusMap {
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

		data.Projects = append(data.Projects, ProjectView{
			Name:        name,
			Status:      s.Status,
			StateClass:  stateClass,
			LastResult:  s.LastResult,
			ResultClass: resultClass,
			TimeAgo:     timeStr,
		})
	}
	h.Manager.Mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) TriggerBuildAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Project string `json:"project"`
	}
	// Try parsing JSON first
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		// Fallback ignored
	}

	if payload.Project == "" {
		payload.Project = r.URL.Query().Get("project")
	}

	if payload.Project == "" {
		http.Error(w, "Project name required", http.StatusBadRequest)
		return
	}

	if success := h.Manager.TriggerBuild(payload.Project); !success {
		http.Error(w, "Project not found", 404)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Build triggered"})
}
