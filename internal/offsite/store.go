package offsite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Bucket = "box_backup"

type Store struct {
	ProjectURL     string
	PublishableKey string
	AccessToken    func() (string, error)
	HTTPClient     *http.Client
	Held           Share
}

type StoredObject struct {
	ID        *string   `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (object StoredObject) isFolder() bool {
	return object.ID == nil
}

func (store Store) Put(ctx context.Context, path string, body []byte) error {
	response, errorValue := store.send(ctx, http.MethodPost, store.objectURL(path), bytes.NewReader(body), map[string]string{
		"Content-Type": "application/octet-stream",
		"x-upsert":     "true",
	})
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return refusalOf(response, "keeping "+path)
	}
	return nil
}

func (store Store) Get(ctx context.Context, path string) ([]byte, error) {
	if held, isHeld := store.Held.read(path); isHeld {
		return held, nil
	}
	response, errorValue := store.send(ctx, http.MethodGet, store.authenticatedObjectURL(path), nil, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, refusalOf(response, "reading "+path)
	}
	return io.ReadAll(response.Body)
}

func (store Store) List(ctx context.Context, prefix string) ([]StoredObject, error) {
	listed := []StoredObject{}
	for offset := 0; ; offset += listPageSize {
		page, errorValue := store.listPage(ctx, prefix, offset)
		if errorValue != nil {
			return nil, errorValue
		}
		listed = append(listed, page...)
		if len(page) < listPageSize {
			return listed, nil
		}
	}
}

const listPageSize = 1000

func (store Store) ListEvery(ctx context.Context, folder string) ([]StoredObject, error) {
	listed, errorValue := store.List(ctx, folder)
	if errorValue != nil {
		return nil, errorValue
	}
	every := []StoredObject{}
	for _, object := range listed {
		if !object.isFolder() {
			every = append(every, object)
			continue
		}
		nested, errorValue := store.ListEvery(ctx, folder+object.Name+"/")
		if errorValue != nil {
			return nil, errorValue
		}
		for _, inside := range nested {
			inside.Name = object.Name + "/" + inside.Name
			every = append(every, inside)
		}
	}
	return every, nil
}

func (store Store) listPage(ctx context.Context, prefix string, offset int) ([]StoredObject, error) {
	body, errorValue := json.Marshal(map[string]any{
		"prefix": prefix,
		"limit":  listPageSize,
		"offset": offset,
		"sortBy": map[string]string{"column": "name", "order": "asc"},
	})
	if errorValue != nil {
		return nil, errorValue
	}
	response, errorValue := store.send(ctx, http.MethodPost, store.storageURL("/object/list/"+Bucket), bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, refusalOf(response, "listing "+prefix)
	}
	var page []StoredObject
	if errorValue := json.NewDecoder(response.Body).Decode(&page); errorValue != nil {
		return nil, fmt.Errorf("listing %s: %w", prefix, errorValue)
	}
	return page, nil
}

func (store Store) Remove(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	body, errorValue := json.Marshal(map[string][]string{"prefixes": paths})
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := store.send(ctx, http.MethodDelete, store.storageURL("/object/"+Bucket), bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return refusalOf(response, fmt.Sprintf("removing %d objects", len(paths)))
	}
	return nil
}

func (store Store) send(ctx context.Context, method, address string, body io.Reader, headers map[string]string) (*http.Response, error) {
	accessToken, errorValue := store.AccessToken()
	if errorValue != nil {
		return nil, fmt.Errorf("reading this box's host session: %w", errorValue)
	}
	request, errorValue := http.NewRequestWithContext(ctx, method, address, body)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("apikey", store.PublishableKey)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	return store.httpClient().Do(request)
}

func (store Store) storageURL(path string) string {
	return strings.TrimRight(store.ProjectURL, "/") + "/storage/v1" + path
}

func (store Store) objectURL(path string) string {
	return store.storageURL("/object/" + Bucket + "/" + escapedPath(path))
}

func (store Store) authenticatedObjectURL(path string) string {
	return store.storageURL("/object/authenticated/" + Bucket + "/" + escapedPath(path))
}

func escapedPath(path string) string {
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func (store Store) httpClient() *http.Client {
	if store.HTTPClient == nil {
		return &http.Client{Timeout: 5 * time.Minute}
	}
	return store.HTTPClient
}

func refusalOf(response *http.Response, doing string) error {
	detail, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	return fmt.Errorf("%s in the company's backups: the central plane answered %d: %s", doing, response.StatusCode, strings.TrimSpace(string(detail)))
}
