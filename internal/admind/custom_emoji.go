package admind

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Custom emoji live in Mattermost, the current registry of record. The web
// resolves :shortcode: bodies against this list at render time, so both
// imported history and freshly sent messages render the same image without any
// per-message tag or backfill. Images are served back through admind on the
// public domain, never exposing the loopback Mattermost origin to the browser.
const customEmojiProxyPrefix = "/emoji/"
const customEmojiCacheTTL = 10 * time.Minute
const customEmojiPageSize = 200

type customEmojiRecord struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type customEmojiListResponse struct {
	Emoji []customEmojiRecord `json:"emoji"`
}

type mattermostCustomEmoji struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var customEmojiCache struct {
	sync.Mutex
	records   []customEmojiRecord
	expiresAt time.Time
}

func (service *Service) handleCustomEmojiList(responseWriter http.ResponseWriter, request *http.Request) {
	if service.webActorEmail(request) == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	records, errorValue := service.customEmojiRecords(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "custom emoji unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, customEmojiListResponse{Emoji: records})
}

func (service *Service) customEmojiRecords(ctx context.Context) ([]customEmojiRecord, error) {
	customEmojiCache.Lock()
	defer customEmojiCache.Unlock()
	if customEmojiCache.records != nil && time.Now().Before(customEmojiCache.expiresAt) {
		return customEmojiCache.records, nil
	}
	fetched, errorValue := service.fetchMattermostCustomEmoji(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	customEmojiCache.records = fetched
	customEmojiCache.expiresAt = time.Now().Add(customEmojiCacheTTL)
	return fetched, nil
}

func (service *Service) fetchMattermostCustomEmoji(ctx context.Context) ([]customEmojiRecord, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	publicBase := service.mediaPublicBase()
	records := []customEmojiRecord{}
	for page := 0; page < 100; page++ {
		var pageEmoji []mattermostCustomEmoji
		path := "/api/v4/emoji?page=" + strconv.Itoa(page) + "&per_page=" + strconv.Itoa(customEmojiPageSize) + "&sort=name"
		if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &pageEmoji); errorValue != nil {
			return nil, errorValue
		}
		for _, emoji := range pageEmoji {
			if emoji.Name == "" || emoji.ID == "" {
				continue
			}
			records = append(records, customEmojiRecord{Name: emoji.Name, URL: publicBase + customEmojiProxyPrefix + emoji.ID})
		}
		if len(pageEmoji) < customEmojiPageSize {
			break
		}
	}
	return records, nil
}

func (service *Service) handleCustomEmojiImage(responseWriter http.ResponseWriter, request *http.Request) {
	if service.webActorEmail(request) == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	emojiID := strings.TrimPrefix(request.URL.Path, customEmojiProxyPrefix)
	if emojiID == "" || strings.Contains(emojiID, "/") {
		http.Error(responseWriter, "invalid emoji", http.StatusBadRequest)
		return
	}
	token, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "custom emoji unavailable", http.StatusBadGateway)
		return
	}
	upstreamURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/emoji/" + url.PathEscape(emojiID) + "/image"
	upstreamRequest, errorValue := http.NewRequestWithContext(request.Context(), http.MethodGet, upstreamURL, nil)
	if errorValue != nil {
		http.Error(responseWriter, "invalid emoji request", http.StatusBadRequest)
		return
	}
	upstreamRequest.Header.Set("Authorization", "Bearer "+token)
	response, errorValue := service.httpClient().Do(upstreamRequest)
	if errorValue != nil {
		http.Error(responseWriter, "emoji_unreachable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	copyHeader(responseWriter, response, "Content-Type")
	copyHeader(responseWriter, response, "Content-Length")
	responseWriter.Header().Set("Cache-Control", "public, max-age=86400")
	responseWriter.WriteHeader(response.StatusCode)
	io.Copy(responseWriter, response.Body)
}
