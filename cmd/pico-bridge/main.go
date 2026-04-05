package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const (
	picoTokenPrefix = "pico-"
	defaultPort     = "8090"
	defaultHome     = "/root/.picoclaw"
)

type pidFileData struct {
	Token string `json:"token"`
	Port  int    `json:"port"`
	Host  string `json:"host"`
}

type picoConfig struct {
	Channels struct {
		Pico struct {
			Token string `json:"token"`
		} `json:"pico"`
	} `json:"channels"`
}

func buildPicoToken(home string) (string, int, error) {
	pidPath := filepath.Join(home, ".picoclaw.pid")
	pidRaw, err := os.ReadFile(pidPath)
	if err != nil {
		return "", 0, fmt.Errorf("read pid file: %w", err)
	}
	var pid pidFileData
	if err := json.Unmarshal(pidRaw, &pid); err != nil {
		return "", 0, fmt.Errorf("parse pid file: %w", err)
	}

	cfgPath := filepath.Join(home, "config.json")
	cfgRaw, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", 0, fmt.Errorf("read config: %w", err)
	}
	var cfg picoConfig
	if err := json.Unmarshal(cfgRaw, &cfg); err != nil {
		return "", 0, fmt.Errorf("parse config: %w", err)
	}

	token := picoTokenPrefix + pid.Token + cfg.Channels.Pico.Token
	return token, pid.Port, nil
}

func handleToken(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			cors(w)
			w.WriteHeader(200)
			return
		}

		token, port, err := buildPicoToken(home)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		cors(w)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":  token,
			"ws_url": fmt.Sprintf("ws://localhost:%d/pico/ws", port),
		})
	}
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func main() {
	port := defaultPort
	if p := os.Getenv("BRIDGE_PORT"); p != "" {
		port = p
	}
	home := defaultHome
	if h := os.Getenv("PICOCLAW_HOME"); h != "" {
		home = h
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/pico/token", handleToken(home))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Fprintf(os.Stderr, "pico-token-server listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
