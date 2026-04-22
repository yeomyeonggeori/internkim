package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// roles.json: {"users": {"mm_user_id": 3, ...}, "default_level": 0}
// Higher level = more access. 0=normal, 1=lead, 2=c-level, 3=owner
type RoleConfig struct {
	Users        map[string]int `json:"users"`
	DefaultLevel int            `json:"default_level"`
}

const (
	minRelevanceScore     = 0.4
	vectorWeight          = 0.7
	keywordWeight         = 0.3
	conversationPurgeDays = 30
	embeddingCacheMax     = 10000
	defaultLevel          = 0
)

var (
	roleConfig     RoleConfig
	sessionsDir    string
	mmURL          string
	mmBotToken     string
	db             *sql.DB
	embeddingKey   string
	embeddingModel string
	httpClient     = &http.Client{Timeout: 30 * time.Second}
)

func main() {
	configPath := envOr("ROLE_MEMORY_CONFIG", "/root/.internkim/roles.json")
	dbPath := envOr("ROLE_MEMORY_DB", "/root/.internkim/role-brain.db")
	sessionsDir = envOr("ZEROCLAW_SESSIONS_DIR", "/root/.zeroclaw/workspace/sessions")
	embeddingKey = loadAPIKey()
	embeddingModel = envOr("EMBEDDING_MODEL", "google/gemini-embedding-2-preview")

	mmURL = loadFile(envOr("MM_URL_FILE", "/root/.internkim/env/mattermost-url"))
	mmBotToken = loadFile(envOr("MM_BOT_TOKEN_FILE", "/root/.internkim/env/bot-token"))

	loadRoleConfig(configPath)

	os.MkdirAll(filepath.Dir(dbPath), 0700)

	var err error
	db, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		fmt.Fprintf(os.Stderr, "role-memory: db open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := initSchema(); err != nil {
		fmt.Fprintf(os.Stderr, "role-memory: schema: %v\n", err)
		os.Exit(1)
	}

	os.Chmod(dbPath, 0600)
	os.Chmod(dbPath+"-wal", 0600)
	os.Chmod(dbPath+"-shm", 0600)

	go hygieneTicker()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		resp := handle(req)
		if resp != nil {
			out, _ := json.Marshal(resp)
			fmt.Fprintf(os.Stdout, "%s\n", out)
		}
	}
}

func initSchema() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id         TEXT PRIMARY KEY,
			key        TEXT UNIQUE NOT NULL,
			content    TEXT NOT NULL,
			category   TEXT NOT NULL DEFAULT 'core',
			embedding  BLOB,
			level      INTEGER NOT NULL DEFAULT 2,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_memories_level ON memories(level);
		CREATE INDEX IF NOT EXISTS idx_memories_cat ON memories(category);
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			key, content, content=memories, content_rowid=rowid
		);
		CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content);
		END;
		CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content);
		END;
		CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content);
			INSERT INTO memories_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content);
		END;
		CREATE TABLE IF NOT EXISTS embedding_cache (
			content_hash TEXT PRIMARY KEY,
			embedding    BLOB NOT NULL,
			created_at   TEXT NOT NULL
		);
	`)
	return err
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadAPIKey() string {
	if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		return v
	}
	keyFile := envOr("OPENROUTER_KEY_FILE", "/root/.internkim/secrets/openrouter-api-key")
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "OPENROUTER_API_KEY=") {
			return strings.TrimPrefix(line, "OPENROUTER_API_KEY=")
		}
	}
	return ""
}

func loadFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func loadRoleConfig(path string) {
	roleConfig.DefaultLevel = defaultLevel
	roleConfig.Users = map[string]int{}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &roleConfig)
}

func handle(req Request) *Response {
	switch req.Method {
	case "initialize":
		return ok(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":   map[string]any{"tools": map[string]any{}},
			"serverInfo":     map[string]any{"name": "role-memory", "version": "2.0.0"},
		})
	case "notifications/initialized":
		return nil
	case "tools/list":
		return ok(req.ID, map[string]any{"tools": toolDefs()})
	case "tools/call":
		var p struct {
			Name string          `json:"name"`
			Args json.RawMessage `json:"arguments"`
		}
		json.Unmarshal(req.Params, &p)
		return ok(req.ID, callTool(p.Name, p.Args))
	default:
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &Error{-32601, "Method not found"}}
	}
}

func ok(id any, result any) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Result: result}
}

func toolDefs() []map[string]any {
	return []map[string]any{
		{
			"name":        "memory_store",
			"description": "Store a memory at the current user's security level. Users at the same or higher level can read it.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":      map[string]any{"type": "string", "description": "Unique key (e.g. user_preference_lang, fact_project_deadline)"},
					"content":  map[string]any{"type": "string", "description": "Content to remember"},
					"category": map[string]any{"type": "string", "enum": []string{"core", "fact", "user_preference", "conversation", "other"}, "default": "core"},
				},
				"required": []string{"key", "content"},
			},
		},
		{
			"name":        "memory_recall",
			"description": "Search memories using hybrid vector + keyword search. Returns memories at or below the current user's security level.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query (semantic + keyword)"},
					"limit": map[string]any{"type": "integer", "default": 5},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "memory_forget",
			"description": "Delete a memory by key. Only works for memories at the current user's security level.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{"type": "string", "description": "Key to forget"},
				},
				"required": []string{"key"},
			},
		},
		{
			"name":        "memory_list",
			"description": "List all memory keys accessible at the current security level.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"category": map[string]any{"type": "string", "description": "Filter by category (optional)"},
				},
			},
		},
		{
			"name":        "memory_get",
			"description": "Get a specific memory by exact key, if accessible at the current security level.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{"type": "string", "description": "Exact key to retrieve"},
				},
				"required": []string{"key"},
			},
		},
	}
}

func callTool(name string, args json.RawMessage) map[string]any {
	userLevel, channelID := resolveSession()
	storeLevel := resolveChannelLevel(channelID, userLevel)

	switch name {
	case "memory_store":
		var a struct {
			Key      string `json:"key"`
			Content  string `json:"content"`
			Category string `json:"category"`
		}
		json.Unmarshal(args, &a)
		if a.Category == "" {
			a.Category = "core"
		}
		if err := store(storeLevel, a.Key, a.Content, a.Category); err != nil {
			return toolErr(err.Error())
		}
		return toolText(fmt.Sprintf("Stored [L%d/%s]: %s", storeLevel, a.Category, a.Key))

	case "memory_recall":
		var a struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		json.Unmarshal(args, &a)
		if a.Limit <= 0 {
			a.Limit = 5
		}
		results, err := recall(userLevel, a.Query, a.Limit)
		if err != nil {
			return toolErr(err.Error())
		}
		if results == "" {
			return toolText("No memories found.")
		}
		return toolText(results)

	case "memory_forget":
		var a struct {
			Key string `json:"key"`
		}
		json.Unmarshal(args, &a)
		n, _ := forget(userLevel, a.Key)
		if n > 0 {
			return toolText(fmt.Sprintf("Forgot: %s", a.Key))
		}
		return toolText(fmt.Sprintf("Not found: %s", a.Key))

	case "memory_list":
		var a struct {
			Category string `json:"category"`
		}
		json.Unmarshal(args, &a)
		results, err := listKeys(userLevel, a.Category)
		if err != nil {
			return toolErr(err.Error())
		}
		if results == "" {
			return toolText("No memories stored.")
		}
		return toolText(results)

	case "memory_get":
		var a struct {
			Key string `json:"key"`
		}
		json.Unmarshal(args, &a)
		result, err := getByKey(userLevel, a.Key)
		if err != nil {
			return toolErr(err.Error())
		}
		if result == "" {
			return toolText(fmt.Sprintf("Not found: %s", a.Key))
		}
		return toolText(result)

	default:
		return toolErr("Unknown tool: " + name)
	}
}

// --- Embedding with cache ---

func contentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:16])
}

func getEmbedding(text string) ([]float32, error) {
	if embeddingKey == "" {
		return nil, nil
	}

	hash := contentHash(text)

	var cached []byte
	err := db.QueryRow("SELECT embedding FROM embedding_cache WHERE content_hash = ?", hash).Scan(&cached)
	if err == nil && len(cached) > 0 {
		return bytesToVec(cached), nil
	}

	body, _ := json.Marshal(map[string]any{
		"model": embeddingModel,
		"input": text,
	})
	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+embeddingKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		msg := string(b)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("embedding API %d: %s", resp.StatusCode, msg)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	emb := result.Data[0].Embedding
	embBytes := vecToBytes(emb)

	db.Exec(`INSERT OR REPLACE INTO embedding_cache (content_hash, embedding, created_at) VALUES (?, ?, ?)`,
		hash, embBytes, time.Now().Format(time.RFC3339))
	db.Exec(`DELETE FROM embedding_cache WHERE content_hash IN (
		SELECT content_hash FROM embedding_cache ORDER BY created_at ASC
		LIMIT MAX(0, (SELECT COUNT(*) FROM embedding_cache) - ?))`,
		embeddingCacheMax)

	return emb, nil
}

func vecToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func bytesToVec(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// --- Storage ---
// key is prefixed with level: "L0:my_key", "L1:my_key"

func store(level int, key, content, category string) error {
	now := time.Now().Format(time.RFC3339)
	id := fmt.Sprintf("L%d_%d", level, time.Now().UnixNano())
	nsKey := fmt.Sprintf("L%d:%s", level, key)

	emb, _ := getEmbedding(content)
	var embBytes []byte
	if emb != nil {
		embBytes = vecToBytes(emb)
	}

	_, err := db.Exec(`
		INSERT INTO memories (id, key, content, category, embedding, level, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			content = excluded.content,
			category = excluded.category,
			embedding = excluded.embedding,
			updated_at = excluded.updated_at`,
		id, nsKey, content, category, embBytes, level, now, now)
	return err
}

// --- Recall ---
// Level 0 can see: level >= 0 (all)
// Level 1 can see: level >= 1
// Level -1 (guest) can see: level >= -1... but we only store at -1, so just own

type scored struct {
	key, content, cat string
	level             int
	score             float32
}

func recall(callerLevel int, query string, limit int) (string, error) {
	vecResults := vectorSearch(callerLevel, query, limit*2)
	ftsResults := ftsSearch(callerLevel, query, limit*2)
	merged := hybridMerge(vecResults, ftsResults, vectorWeight, keywordWeight, limit)

	if len(merged) == 0 {
		return recallLike(callerLevel, query, limit)
	}

	var sb strings.Builder
	for _, r := range merged {
		fmt.Fprintf(&sb, "- %s: %s\n", stripPrefix(r.key), r.content)
	}
	return sb.String(), nil
}

func stripPrefix(key string) string {
	if idx := strings.Index(key, ":"); idx >= 0 {
		return key[idx+1:]
	}
	return key
}

func vectorSearch(callerLevel int, query string, limit int) []scored {
	qEmb, err := getEmbedding(query)
	if err != nil || qEmb == nil {
		return nil
	}

	rows, err := db.Query(`
		SELECT key, content, category, level, embedding
		FROM memories
		WHERE level <= ? AND embedding IS NOT NULL`, callerLevel)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []scored
	for rows.Next() {
		var key, content, cat string
		var lvl int
		var embBlob []byte
		rows.Scan(&key, &content, &cat, &lvl, &embBlob)
		if len(embBlob) == 0 {
			continue
		}
		sim := cosineSimilarity(qEmb, bytesToVec(embBlob))
		if sim >= minRelevanceScore {
			results = append(results, scored{key, content, cat, lvl, sim})
		}
	}

	sortScored(results)
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func ftsSearch(callerLevel int, query string, limit int) []scored {
	words := strings.Fields(query)
	if len(words) == 0 {
		return nil
	}
	ftsTerms := make([]string, len(words))
	for i, w := range words {
		ftsTerms[i] = `"` + strings.ReplaceAll(w, `"`, `""`) + `"`
	}
	ftsQuery := strings.Join(ftsTerms, " OR ")

	rows, err := db.Query(`
		SELECT m.key, m.content, m.category, m.level, bm25(memories_fts) as score
		FROM memories_fts f
		JOIN memories m ON m.rowid = f.rowid
		WHERE memories_fts MATCH ?
		  AND m.level <= ?
		ORDER BY score
		LIMIT ?`, ftsQuery, callerLevel, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []scored
	for rows.Next() {
		var key, content, cat string
		var lvl int
		var sc float64
		rows.Scan(&key, &content, &cat, &lvl, &sc)
		results = append(results, scored{key, content, cat, lvl, float32(-sc)})
	}
	return results
}

func hybridMerge(vec, fts []scored, vecW, ftsW float32, limit int) []scored {
	type entry struct {
		s      scored
		vScore float32
		fScore float32
	}
	m := map[string]*entry{}

	for _, v := range vec {
		m[v.key] = &entry{s: v, vScore: v.score}
	}
	for _, f := range fts {
		if e, ok := m[f.key]; ok {
			e.fScore = f.score
		} else {
			m[f.key] = &entry{s: f, fScore: f.score}
		}
	}

	var maxV, maxF float32
	for _, e := range m {
		if e.vScore > maxV {
			maxV = e.vScore
		}
		if e.fScore > maxF {
			maxF = e.fScore
		}
	}

	var results []scored
	for _, e := range m {
		nv := e.vScore
		if maxV > 0 {
			nv /= maxV
		}
		nf := e.fScore
		if maxF > 0 {
			nf /= maxF
		}
		e.s.score = vecW*nv + ftsW*nf
		results = append(results, e.s)
	}

	sortScored(results)
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func sortScored(s []scored) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].score > s[j-1].score; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// --- List ---

func listKeys(callerLevel int, category string) (string, error) {
	q := `SELECT key, category, level FROM memories WHERE level <= ?`
	args := []any{callerLevel}
	if category != "" {
		q += " AND category = ?"
		args = append(args, category)
	}
	q += " ORDER BY level, category, key"

	rows, err := db.Query(q, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var key, cat string
		var lvl int
		rows.Scan(&key, &cat, &lvl)
		fmt.Fprintf(&sb, "[L%d/%s] %s\n", lvl, cat, stripPrefix(key))
	}
	return sb.String(), nil
}

// --- Get ---

func getByKey(callerLevel int, key string) (string, error) {
	// Try each level from caller's level upward
	var content, cat string
	var lvl int
	err := db.QueryRow(`
		SELECT content, category, level FROM memories
		WHERE key LIKE ? AND level <= ?
		ORDER BY level ASC LIMIT 1`,
		"%:"+key, callerLevel).Scan(&content, &cat, &lvl)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("[L%d/%s] %s: %s", lvl, cat, key, content), nil
}

// --- LIKE fallback ---

func recallLike(callerLevel int, query string, limit int) (string, error) {
	words := strings.Fields(query)
	if len(words) == 0 {
		return "", nil
	}
	args := []any{callerLevel}
	likeConds := make([]string, len(words))
	for i, w := range words {
		likeConds[i] = "(content LIKE ? OR key LIKE ?)"
		args = append(args, "%"+w+"%", "%"+w+"%")
	}
	args = append(args, limit)

	q := fmt.Sprintf(`
		SELECT key, content, category, level
		FROM memories WHERE level <= ? AND (%s)
		ORDER BY updated_at DESC LIMIT ?`,
		strings.Join(likeConds, " OR "))

	rows, err := db.Query(q, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var key, content, cat string
		var lvl int
		rows.Scan(&key, &content, &cat, &lvl)
		fmt.Fprintf(&sb, "- %s: %s\n", stripPrefix(key), content)
	}
	return sb.String(), nil
}

// --- Forget ---

func forget(callerLevel int, key string) (int64, error) {
	nsKey := fmt.Sprintf("L%d:%s", callerLevel, key)
	res, err := db.Exec("DELETE FROM memories WHERE key = ? AND level = ?", nsKey, callerLevel)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- Hygiene ---

func hygieneTicker() {
	runHygiene()
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		runHygiene()
	}
}

func runHygiene() {
	cutoff := time.Now().AddDate(0, 0, -conversationPurgeDays).Format(time.RFC3339)
	db.Exec(`DELETE FROM memories WHERE category = 'conversation' AND updated_at < ?`, cutoff)
}

// --- Session resolution ---
// Returns (userLevel, channelID) from the most recent session file.
// Filename format: mattermost_{channelId}_{msgId}_{senderId}.jsonl

func resolveSession() (int, string) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return roleConfig.DefaultLevel, ""
	}

	var latest string
	var latestTime time.Time
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "mattermost_") || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = e.Name()
		}
	}
	if latest == "" {
		return roleConfig.DefaultLevel, ""
	}

	base := strings.TrimSuffix(latest, ".jsonl")
	parts := strings.SplitN(base, "_", 4)
	if len(parts) < 4 {
		return roleConfig.DefaultLevel, ""
	}
	return senderToLevel(parts[3]), parts[1]
}

// resolveChannelLevel queries MM API for channel members and returns
// the minimum level among them. This determines the store visibility:
// memories stored in a channel are visible to anyone at that level or above.
func resolveChannelLevel(channelID string, fallback int) int {
	if channelID == "" || mmURL == "" || mmBotToken == "" {
		return fallback
	}

	req, err := http.NewRequest("GET", mmURL+"/api/v4/channels/"+channelID+"/members?per_page=200", nil)
	if err != nil {
		return fallback
	}
	req.Header.Set("Authorization", "Bearer "+mmBotToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fallback
	}

	var members []struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return fallback
	}

	if len(members) == 0 {
		return fallback
	}

	minLevel := math.MaxInt
	for _, m := range members {
		lvl := senderToLevel(m.UserID)
		if lvl < minLevel {
			minLevel = lvl
		}
	}
	return minLevel
}

func senderToLevel(senderID string) int {
	if lvl, ok := roleConfig.Users[senderID]; ok {
		return lvl
	}
	return roleConfig.DefaultLevel
}

// --- output helpers ---

func toolText(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	}
}

func toolErr(msg string) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []map[string]any{{"type": "text", "text": "Error: " + msg}},
	}
}

// suppress unused import
var _ = strconv.Itoa
