package model

import "time"

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

type ProjectStatus struct {
	LastRun    time.Time
	Status     string // "Idle", "Building", "Success", "Failed"
	LastResult string // "Success" or "Rollback"
}
