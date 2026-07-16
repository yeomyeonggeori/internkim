package admind

import (
	"context"
	"fmt"
	"strings"
)

func recoverCanonicalETagAfterCalDAVPut(ctx context.Context, client *outboundCalDAVClient, objectPath string, responseETag string) (string, error) {
	if etag := strings.TrimSpace(responseETag); etag != "" {
		return etag, nil
	}
	remoteObject, errorValue := client.getCalendarObject(ctx, objectPath)
	if errorValue != nil {
		return "", fmt.Errorf("recover canonical ETag after CalDAV PUT %s: %v", objectPath, errorValue)
	}
	etag := strings.TrimSpace(remoteObject.ETag)
	if etag == "" {
		return "", fmt.Errorf("recover canonical ETag after CalDAV PUT %s: GET response ETag is empty", objectPath)
	}
	return etag, nil
}
