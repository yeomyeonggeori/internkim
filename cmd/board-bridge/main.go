package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	picoTokenPrefix = "pico-"
	defaultPort     = "8080"
	defaultHome     = "/root/.zeroclaw"
	defaultWebRoot  = "/var/www"
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
	pidRaw, err := os.ReadFile(filepath.Join(home, ".zeroclaw.pid"))
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

func restartZeroclaw() error {
	exec.Command("killall", "zeroclaw").Run()
	exec.Command("rm", "-f", "/root/.zeroclaw/.zeroclaw.pid").Run()
	exec.Command("sh", "-c", "systemctl start zeroclaw 2>/dev/null").Run()

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get("http://localhost:18790/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				if _, err := os.Stat("/root/.zeroclaw/.zeroclaw.pid"); err == nil {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("zeroclaw failed to start")
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
			"ws_url": fmt.Sprintf("ws://localhost:%d/board/ws", port),
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
			m := getModel(cfg)
			m = strings.TrimPrefix(m, "openrouter/")
			json.NewEncoder(w).Encode(map[string]string{"model": m})

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
			setModel(cfg, ensurePrefix(req.Model))
			if err := writeConfig(home, cfg); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"old": old, "new": req.Model})

		default:
			http.Error(w, "method not allowed", 405)
		}
	}
}

func ensurePrefix(model string) string {
	if strings.HasPrefix(model, "openrouter/") {
		return model
	}
	return "openrouter/" + model
}

func handleHistory(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(200)
			return
		}
		sid := strings.TrimPrefix(r.URL.Path, "/board/history/")
		if sid == "" {
			http.Error(w, "session_id required", 400)
			return
		}

		sessDir := filepath.Join(home, "workspace", "sessions")
		// Session file pattern: agent_main_pico_direct_pico_{session_id}.jsonl
		pattern := filepath.Join(sessDir, "agent_main_pico_direct_pico_"+sid+".jsonl")
		raw, err := os.ReadFile(pattern)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"messages": []any{}})
			return
		}

		var messages []map[string]string
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if line == "" {
				continue
			}
			var msg map[string]string
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			role := msg["role"]
			content := strings.TrimSpace(msg["content"])
			if (role == "user" || role == "assistant") && content != "" {
				messages = append(messages, map[string]string{
					"role":    role,
					"content": content,
				})
			}
		}
		if messages == nil {
			messages = []map[string]string{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"messages": messages})
	}
}

func handleFiles(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relPath := strings.TrimPrefix(r.URL.Path, "/board/files/")
		if relPath == "" || strings.Contains(relPath, "..") {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		filePath := filepath.Join(home, "workspace", relPath)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
		http.ServeFile(w, r, filePath)
	}
}

func handleWSProxy(w http.ResponseWriter, r *http.Request) {
	backend, err := net.Dial("tcp", "localhost:18790")
	if err != nil {
		http.Error(w, "backend unavailable", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		backend.Close()
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}

	// Forward the original request to backend
	r.URL.Path = "/board/ws"
	r.URL.Scheme = "http"
	r.URL.Host = "localhost:18790"
	r.Header.Set("Host", "localhost:18790")
	if err := r.Write(backend); err != nil {
		backend.Close()
		http.Error(w, "failed to write to backend", http.StatusBadGateway)
		return
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		backend.Close()
		return
	}

	go func() { io.Copy(backend, client); backend.Close() }()
	go func() { io.Copy(client, backend); client.Close() }()
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
	if h := os.Getenv("ZEROCLAW_HOME"); h != "" {
		home = h
	}

	webRoot := defaultWebRoot
	if w := os.Getenv("WEB_ROOT"); w != "" {
		webRoot = w
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/board/ws", handleWSProxy)
	mux.HandleFunc("/board/token", handleToken(home))
	mux.HandleFunc("/board/model", handleModel(home))
	mux.HandleFunc("/board/history/", handleHistory(home))
	mux.HandleFunc("/board/me", func(w http.ResponseWriter, r *http.Request) {
		email := r.Header.Get("Cf-Access-Authenticated-User-Email")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"email": email})
	})
	mux.HandleFunc("/board/files/", handleFiles(home))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	// Static file serving with SPA fallback
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(webRoot, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		// SPA fallback
		http.ServeFile(w, r, filepath.Join(webRoot, "index.html"))
	})

	fmt.Fprintf(os.Stderr, "board-bridge listening on :%s (web: %s)\n", port, webRoot)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
