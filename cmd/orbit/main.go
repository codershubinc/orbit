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
   ____  ____  ____  __  ______
  / __ \/ __ \/ __ \/  |/  /_  __/
 / /_/ / /_/ / /_/ / /|_/ / / /   
 \____/_/ |_/_____/_/  /_/ /_/    
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
	mux.HandleFunc("/", h.Dashboard)

	// 4. Start Server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🔭 [Orbit] Dashboard live at http://localhost:%d", cfg.Port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
