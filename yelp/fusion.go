package yelp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// fusion.go is the transport for Yelp's official Fusion API (api.yelp.com/v3).
// Every fusion call attaches the operator's bearer key, reads JSON, and maps a
// Fusion error envelope to the same sentinels the web plane uses. This client
// sends only documented GET endpoints; it registers nothing and writes nothing.

// fusionError is the Fusion error envelope: {"error":{"code":"...","description":"..."}}.
type fusionError struct {
	Error *struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"error"`
}

// fusionGet calls a Fusion endpoint and unmarshals the JSON into out. path is the
// endpoint path under /v3 (for example "businesses/search"); q is the query.
func (c *Client) fusionGet(ctx context.Context, path string, q url.Values, out any) error {
	u := strings.TrimRight(c.FusionBase, "/") + "/v3/" + strings.TrimPrefix(path, "/")
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	ck := "fusion:" + u
	body, ok := c.cacheGet(ck)
	if !ok {
		header := http.Header{}
		header.Set("Authorization", "Bearer "+c.apiKey)
		header.Set("Accept", "application/json")

		var lastErr error
		for attempt := 0; attempt <= c.Retries; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
			b, retry, err := c.do(ctx, http.MethodGet, u, header, nil)
			if err == nil {
				body = b
				lastErr = nil
				break
			}
			lastErr = err
			if !retry {
				return classifyFusion(b, err)
			}
		}
		if lastErr != nil {
			return lastErr
		}
		c.cache.put(ck, body)
	}

	// A 200 can still carry a Fusion error envelope.
	var fe fusionError
	if json.Unmarshal(body, &fe) == nil && fe.Error != nil {
		return classifyFusionCode(fe.Error.Code)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode fusion %s: %w", path, err)
	}
	return nil
}

// cacheGet reads the response cache unless --refresh is set.
func (c *Client) cacheGet(key string) ([]byte, bool) {
	if c.refresh {
		return nil, false
	}
	return c.cache.get(key)
}

// classifyFusion maps a transport error (already a sentinel from do) through, or
// reads the body for an error envelope when the status was a 4xx do turned into a
// generic error.
func classifyFusion(body []byte, err error) error {
	var fe fusionError
	if json.Unmarshal(body, &fe) == nil && fe.Error != nil {
		return classifyFusionCode(fe.Error.Code)
	}
	return err
}

// classifyFusionCode maps a Fusion error code to a sentinel.
func classifyFusionCode(code string) error {
	switch strings.ToUpper(code) {
	case "BUSINESS_NOT_FOUND", "NOT_FOUND", "RESOURCE_NOT_FOUND":
		return ErrNotFound
	case "TOO_MANY_REQUESTS_PER_SECOND", "ACCESS_LIMIT_REACHED", "RATE_LIMIT":
		return ErrRateLimited
	case "TOKEN_MISSING", "TOKEN_INVALID", "VALIDATION_ERROR", "UNAUTHORIZED":
		return ErrKeyRejected
	default:
		return ErrKeyRejected
	}
}
