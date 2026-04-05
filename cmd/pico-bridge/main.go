package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	picoclaw = "/usr/local/bin/picoclaw"
	port     = "8090"
	ansiRe   = regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\[K`)
)

type chatRequest struct {
	Messages  []message `json:"messages"`
	SessionID string    `json:"session_id,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Choices []choice `json:"choices"`
}

type choice struct {
	Index        int     `json:"index"`
	Message      message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

func callAgent(text, sessionID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, picoclaw, "agent", "-m", text, "-s", sessionID)
	out, _ := cmd.CombinedOutput()
	return cleanOutput(string(out))
}

func cleanOutput(raw string) string {
	raw = ansiRe.ReplaceAllString(raw, "")
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.ContainsAny(line, "██╗╝═") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		// Strip 🦞 prefix if present
		trimmed = strings.TrimPrefix(trimmed, "🦞")
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasPrefix(trimmed, "ERR ") || strings.HasPrefix(trimmed, "WRN ") ||
			strings.HasPrefix(trimmed, "INF ") || strings.HasPrefix(trimmed, "DBG ") ||
			strings.Contains(trimmed, "Usage:") || strings.Contains(trimmed, "Flags:") ||
			strings.Contains(trimmed, "picoclaw agent") ||
			strings.HasPrefix(trimmed, "-m, ") || strings.HasPrefix(trimmed, "-d, ") ||
			strings.HasPrefix(trimmed, "-h, ") || strings.HasPrefix(trimmed, "-s, ") {
			continue
		}
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) == 0 {
		return "(no response)"
	}
	return strings.Join(lines, "\n")
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		corsHeaders(w)
		w.WriteHeader(200)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	var text string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			text = req.Messages[i].Content
			break
		}
	}
	if text == "" {
		http.Error(w, "no user message", 400)
		return
	}

	sid := req.SessionID
	if sid == "" {
		sid = fmt.Sprintf("web:%d", time.Now().UnixMilli())
	}

	content := callAgent(text, sid)

	resp := chatResponse{
		ID:     fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli()),
		Object: "chat.completion",
		Choices: []choice{{
			Index:        0,
			Message:      message{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
	}

	corsHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func corsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	if p := os.Getenv("BRIDGE_PORT"); p != "" {
		port = p
	}
	if p := os.Getenv("PICOCLAW_BIN"); p != "" {
		picoclaw = p
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handleChat)
	mux.HandleFunc("/health", handleHealth)

	fmt.Fprintf(os.Stderr, "pico-bridge listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
