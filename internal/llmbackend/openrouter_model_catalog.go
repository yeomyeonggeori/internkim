package llmbackend

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const openRouterModelCatalogRefreshInterval = time.Hour

type openRouterModelCatalogEntry struct {
	contextLengthByModel map[string]int64
	fetchedAt            time.Time
}

type openRouterModelCatalog struct {
	mutex        sync.Mutex
	entriesByURL map[string]openRouterModelCatalogEntry
}

var sharedOpenRouterModelCatalog openRouterModelCatalog

func (catalog *openRouterModelCatalog) contextWindowTokens(modelsURL string, httpClient *http.Client, modelName string) int64 {
	catalog.mutex.Lock()
	defer catalog.mutex.Unlock()
	if catalog.entriesByURL == nil {
		catalog.entriesByURL = map[string]openRouterModelCatalogEntry{}
	}
	entry, isCached := catalog.entriesByURL[modelsURL]
	if !isCached || time.Since(entry.fetchedAt) > openRouterModelCatalogRefreshInterval {
		entry = openRouterModelCatalogEntry{
			contextLengthByModel: fetchOpenRouterModelContextLengths(modelsURL, httpClient),
			fetchedAt:            time.Now(),
		}
		catalog.entriesByURL[modelsURL] = entry
	}
	return entry.contextLengthByModel[modelName]
}

func fetchOpenRouterModelContextLengths(modelsURL string, httpClient *http.Client) map[string]int64 {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	requestClient := *httpClient
	requestClient.Timeout = 5 * time.Second
	response, errorValue := requestClient.Get(modelsURL)
	if errorValue != nil {
		return map[string]int64{}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return map[string]int64{}
	}
	var document struct {
		Data []struct {
			ID            string `json:"id"`
			ContextLength int64  `json:"context_length"`
		} `json:"data"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		return map[string]int64{}
	}
	contextLengthByModel := make(map[string]int64, len(document.Data))
	for _, model := range document.Data {
		if model.ID != "" && model.ContextLength > 0 {
			contextLengthByModel[model.ID] = model.ContextLength
		}
	}
	return contextLengthByModel
}
