package main

import (
	"fmt"
	"log"
	"net/http"

	"orbit/internal/config"
	"orbit/internal/handler"
	"orbit/internal/service"
)

func main() {
	// 0. Print Banner
	fmt.Println(`
   ____  ____  ____  _____ ______
  / __ \/ __ \/ __ )/  _// ____/
 / / / / /_/ / __  |/ / / /     
/ /_/ / _, _/ /_/ // / / /___   
\____/_/ |_/_____/___/ \____/   
     v2.0 • Modular Architecture
	`)

	// 1. Load Config
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("❌ [Orbit] Could not read config.json: %v", err)
	}

	// 2. Initialize Core Components
	manager := service.NewManager(cfg)
	h := handler.NewHandler(manager)

	// 3. Setup Routes
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("/webhook", h.Webhook)
	mux.HandleFunc("/build", h.Build) // <--- Manual Build
	mux.HandleFunc("/config", h.Config)
	// serve html file of dashboard
	mux.HandleFunc("/", h.Dashboard)

	// API Routes
	mux.HandleFunc("GET /api/dashboard", h.GetDashboardAPI)
	mux.HandleFunc("POST /api/build", h.TriggerBuildAPI)
	mux.HandleFunc("GET /api/config", h.GetConfigAPI)
	mux.HandleFunc("PUT /api/config/global", h.UpdateGlobalConfigAPI)
	mux.HandleFunc("POST /api/projects", h.CreateProjectAPI)
	mux.HandleFunc("PUT /api/projects/{name}", h.UpdateProjectAPI)
	mux.HandleFunc("DELETE /api/projects/{name}", h.DeleteProjectAPI)
	mux.HandleFunc("GET /api/machine-info", h.MachineInfoApi)

	// 4. Start Server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🔭 [Orbit] Dashboard live at http://localhost:%d", cfg.Port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
