package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"orbit/internal/config"
	"orbit/internal/model"
	"orbit/internal/service"
)

type Handler struct {
	Manager *service.Manager
}

func NewHandler(m *service.Manager) *Handler {
	return &Handler{Manager: m}
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	load := h.Manager.GetSystemLoad()

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

		data.Projects[name] = ProjectView{
			Status:      s.Status,
			StateClass:  stateClass,
			LastResult:  s.LastResult,
			ResultClass: resultClass,
			TimeAgo:     timeStr,
		}
	}
	h.Manager.Mutex.Unlock()

	tmpl, err := template.ParseFiles("web/templates/dashboard.html")
	if err != nil {
		http.Error(w, "Could not load dashboard template", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}
	tmpl.Execute(w, data)
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

		// Save Logic
		// Ideally pass a path, for now hardcoding or using global config path logic if we had one
		if err := config.Save("config.json", h.Manager.Config); err != nil {
			http.Error(w, "Failed to write config file", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/config", http.StatusSeeOther)
		return
	}

	// GET Request - Render Template
	// Re-read config to ensure fresh state if edited manually?
	// Or just display current memory state. For now memory state matches requirements.

	// We might want to pass the map explicitly so template renders range correctly
	tmpl, err := template.ParseFiles("web/templates/config.html")
	if err != nil {
		http.Error(w, "Could not load config template", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}
	tmpl.Execute(w, h.Manager.Config)
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
