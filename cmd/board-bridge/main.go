package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPort    = "8080"
	defaultHome    = "/root/.zeroclaw"
	defaultWebRoot = "/var/www"
	zeroclawPort   = "8080"
)

// zeroclawGatewayURL returns the ZeroClaw gateway base URL.
// ZeroClaw gateway runs on port 8080 internally, board-bridge runs on the
// same port exposed to Cloudflare. We use an internal port for ZeroClaw.
func zeroclawGatewayURL() string {
	port := os.Getenv("ZEROCLAW_GATEWAY_PORT")
	if port == "" {
		port = "18790"
	}
	return "http://localhost:" + port
}

func readTOMLValue(home, key string) string {
	path := filepath.Join(home, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+" =") || strings.HasPrefix(line, key+"=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				v := strings.TrimSpace(parts[1])
				v = strings.Trim(v, `"`)
				return v
			}
		}
	}
	return ""
}

func writeTOMLValue(home, key, value string) error {
	path := filepath.Join(home, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" =") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = key + ` = "` + value + `"`
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+` = "`+value+`"`)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

func webhookSecret(home string) string {
	// Read from config.toml: channels_config.webhook.secret
	// or fall back to reading the hardcoded token we set during setup
	secret := readTOMLValue(home, "token")
	if secret == "" {
		secret = "quickclaw"
	}
	return secret
}

func restartZeroclaw() error {
	exec.Command("systemctl", "restart", "zeroclaw").Run()
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get(zeroclawGatewayURL() + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
	}
	return fmt.Errorf("zeroclaw failed to start")
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Webhook-Secret")
}

// handleToken returns the webhook secret so the web UI can authenticate.
func handleToken(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			cors(w)
			w.WriteHeader(200)
			return
		}
		cors(w)
		w.Header().Set("Content-Type", "application/json")
		secret := webhookSecret(home)
		json.NewEncoder(w).Encode(map[string]any{
			"token":       secret,
			"webhook_url": zeroclawGatewayURL() + "/webhook",
		})
	}
}

// handleChat proxies POST {message} to ZeroClaw gateway /webhook and returns the response.
// The web UI sends messages here; board-bridge forwards to ZeroClaw and streams the reply.
func handleChat(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(200)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", 500)
			return
		}

		// Forward to ZeroClaw gateway
		secret := webhookSecret(home)
		req, err := http.NewRequest("POST", zeroclawGatewayURL()+"/webhook", bytes.NewReader(body))
		if err != nil {
			http.Error(w, "request error", 500)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Secret", secret)

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "zeroclaw unreachable: "+err.Error(), 502)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// handleChatStream uses ZeroClaw gateway SSE streaming if available,
// otherwise falls back to a non-streaming request.
func handleChatStream(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(200)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", 500)
			return
		}

		secret := webhookSecret(home)
		req, err := http.NewRequest("POST", zeroclawGatewayURL()+"/webhook", bytes.NewReader(body))
		if err != nil {
			http.Error(w, "request error", 500)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Secret", secret)
		req.Header.Set("Accept", "text/event-stream")

		client := &http.Client{Timeout: 300 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "zeroclaw unreachable: "+err.Error(), 502)
			return
		}
		defer resp.Body.Close()

		// Pass through SSE headers if ZeroClaw supports streaming
		if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(200)
			flusher, _ := w.(http.Flusher)
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				fmt.Fprintf(w, "%s\n", scanner.Text())
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}

		// Non-streaming fallback
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func handleModel(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(200)
			return
		}
		switch r.Method {
		case http.MethodGet:
			model := readTOMLValue(home, "default_model")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"model": model})

		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
				http.Error(w, "invalid request", 400)
				return
			}
			old := readTOMLValue(home, "default_model")
			if err := writeTOMLValue(home, "default_model", req.Model); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			restartZeroclaw()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"old": old, "new": req.Model})

		default:
			http.Error(w, "method not allowed", 405)
		}
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
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(filePath)))
		http.ServeFile(w, r, filePath)
	}
}

func handleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Header.Get("Cf-Access-Authenticated-User-Email")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"email": email})
	}
}

func handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Forward to ZeroClaw health check
		resp, err := http.Get(zeroclawGatewayURL() + "/health")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(502)
			json.NewEncoder(w).Encode(map[string]string{"status": "zeroclaw_unreachable"})
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
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

	// ZeroClaw gateway proxy endpoints
	mux.HandleFunc("/board/chat", handleChat(home))
	mux.HandleFunc("/board/chat/stream", handleChatStream(home))
	mux.HandleFunc("/board/token", handleToken(home))
	mux.HandleFunc("/board/model", handleModel(home))
	mux.HandleFunc("/board/files/", handleFiles(home))
	mux.HandleFunc("/board/me", handleMe())

	// Health check (forwards to ZeroClaw)
	mux.HandleFunc("/health", handleHealth())

	// Static file serving with SPA fallback
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(webRoot, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(webRoot, "index.html"))
	})

	fmt.Fprintf(os.Stderr, "board-bridge listening on :%s (web: %s, zeroclaw: %s)\n",
		port, webRoot, zeroclawGatewayURL())
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
