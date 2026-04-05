package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

func readConfig(home string) (map[string]any, error) {
	raw, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	return cfg, json.Unmarshal(raw, &cfg)
}

func writeConfig(home string, cfg map[string]any) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, "config.json"), b, 0644)
}

func buildPicoToken(home string) (string, int, error) {
	pidRaw, err := os.ReadFile(filepath.Join(home, ".picoclaw.pid"))
	if err != nil {
		return "", 0, fmt.Errorf("read pid file: %w", err)
	}
	var pid pidFileData
	if err := json.Unmarshal(pidRaw, &pid); err != nil {
		return "", 0, fmt.Errorf("parse pid file: %w", err)
	}

	cfg, err := readConfig(home)
	if err != nil {
		return "", 0, err
	}

	picoToken := ""
	if ch, ok := cfg["channels"].(map[string]any); ok {
		if pico, ok := ch["pico"].(map[string]any); ok {
			picoToken, _ = pico["token"].(string)
		}
	}
	return picoTokenPrefix + pid.Token + picoToken, pid.Port, nil
}

func getModel(cfg map[string]any) string {
	models, _ := cfg["model_list"].([]any)
	if len(models) == 0 {
		return ""
	}
	m, _ := models[0].(map[string]any)
	s, _ := m["model"].(string)
	return s
}

func setModel(cfg map[string]any, model string) {
	models, _ := cfg["model_list"].([]any)
	if len(models) == 0 {
		return
	}
	m, _ := models[0].(map[string]any)
	m["model"] = model
	models[0] = m
	cfg["model_list"] = models
}

func restartPicoclaw() {
	exec.Command("killall", "picoclaw").Run()
	exec.Command("rm", "-f", "/root/.picoclaw/.picoclaw.pid").Run()
	exec.Command("sh", "-c", "/etc/init.d/S99picoclaw start 2>/dev/null &").Run()
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

func handleModel(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(200)
			return
		}

		cfg, err := readConfig(home)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"model": getModel(cfg)})

		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
				http.Error(w, "invalid request", 400)
				return
			}
			old := getModel(cfg)
			setModel(cfg, req.Model)
			if err := writeConfig(home, cfg); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			restartPicoclaw()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"old": old, "new": req.Model})

		default:
			http.Error(w, "method not allowed", 405)
		}
	}
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
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
	mux.HandleFunc("/pico/model", handleModel(home))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Fprintf(os.Stderr, "pico-token-server listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
