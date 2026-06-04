package releaseset

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type R2Configuration struct {
	AccountID       string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	HTTPClient      *http.Client
}

type R2Client struct {
	configuration R2Configuration
}

func NewR2Client(configuration R2Configuration) (R2Client, error) {
	if strings.TrimSpace(configuration.AccountID) == "" {
		return R2Client{}, errors.New("R2 account id is required")
	}
	if strings.TrimSpace(configuration.Bucket) == "" {
		return R2Client{}, errors.New("R2 bucket is required")
	}
	if strings.TrimSpace(configuration.AccessKeyID) == "" {
		return R2Client{}, errors.New("R2 access key id is required")
	}
	if strings.TrimSpace(configuration.SecretAccessKey) == "" {
		return R2Client{}, errors.New("R2 secret access key is required")
	}
	if strings.TrimSpace(configuration.PublicBaseURL) == "" {
		return R2Client{}, errors.New("R2 public base URL is required")
	}
	if configuration.HTTPClient == nil {
		configuration.HTTPClient = http.DefaultClient
	}
	return R2Client{configuration: configuration}, nil
}

func (client R2Client) PublicURL(objectKey string) string {
	publicBaseURL := strings.TrimRight(strings.TrimSpace(client.configuration.PublicBaseURL), "/")
	if publicBaseURL == "" {
		return ""
	}
	return publicBaseURL + "/" + strings.TrimLeft(path.Clean(objectKey), "/")
}

func (client R2Client) PutObject(objectKey string, document []byte, contentType string) error {
	objectURL := client.objectURL(objectKey)
	request, errorValue := http.NewRequest(http.MethodPut, objectURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(contentType) != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.Header.Set("Content-Length", fmt.Sprintf("%d", len(document)))
	client.signRequest(request, document)
	response, errorValue := client.configuration.HTTPClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("R2 put object %s failed: HTTP %d %s", objectKey, response.StatusCode, strings.TrimSpace(string(body)))
}

func (client R2Client) objectURL(objectKey string) string {
	cleanObjectKey := strings.TrimLeft(path.Clean(objectKey), "/")
	return fmt.Sprintf(
		"https://%s.r2.cloudflarestorage.com/%s/%s",
		strings.TrimSpace(client.configuration.AccountID),
		url.PathEscape(strings.TrimSpace(client.configuration.Bucket)),
		escapeObjectKey(cleanObjectKey),
	)
}

func (client R2Client) signRequest(request *http.Request, document []byte) {
	requestTime := time.Now().UTC()
	payloadHash := sha256Hex(document)
	dateValue := requestTime.Format("20060102")
	dateTimeValue := requestTime.Format("20060102T150405Z")
	credentialScope := dateValue + "/auto/s3/aws4_request"
	request.Header.Set("Host", request.URL.Host)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	request.Header.Set("X-Amz-Date", dateTimeValue)
	canonicalRequest := strings.Join([]string{
		request.Method,
		request.URL.EscapedPath(),
		"",
		"host:" + request.URL.Host,
		"x-amz-content-sha256:" + payloadHash,
		"x-amz-date:" + dateTimeValue,
		"",
		"host;x-amz-content-sha256;x-amz-date",
		payloadHash,
	}, "\n")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		dateTimeValue,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(signingKey(client.configuration.SecretAccessKey, dateValue).sign([]byte(stringToSign)))
	request.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=%s",
		client.configuration.AccessKeyID,
		credentialScope,
		signature,
	))
}

type signingKeyBytes []byte

func signingKey(secretAccessKey string, dateValue string) signingKeyBytes {
	dateKey := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(dateValue))
	regionKey := hmacSHA256(dateKey, []byte("auto"))
	serviceKey := hmacSHA256(regionKey, []byte("s3"))
	return hmacSHA256(serviceKey, []byte("aws4_request"))
}

func (key signingKeyBytes) sign(document []byte) []byte {
	return hmacSHA256(key, document)
}

func hmacSHA256(key []byte, document []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(document)
	return mac.Sum(nil)
}

func sha256Hex(document []byte) string {
	hash := sha256.Sum256(document)
	return hex.EncodeToString(hash[:])
}

func escapeObjectKey(objectKey string) string {
	parts := strings.Split(objectKey, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
