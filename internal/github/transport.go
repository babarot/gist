package github

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/babarot/gist/v2/internal/logging"
)

// transport logs every request at debug, without bodies, with what GitHub
// says of the rate limit, and sends the ETag the context carries.
type transport struct {
	next http.RoundTripper
	log  *slog.Logger
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if etag, ok := req.Context().Value(etagKey{}).(string); ok && etag != "" {
		req = req.Clone(req.Context())
		req.Header.Set("If-None-Match", etag)
	}
	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	attrs := []any{"method", req.Method, "path", req.URL.Path, logging.KeyDurationMS, time.Since(start).Milliseconds()}
	if err != nil {
		t.log.Debug("github request failed", append(attrs, "err", err.Error())...)
		return resp, err
	}
	t.log.Debug("github request", append(attrs, "status", resp.StatusCode,
		logging.KeyGHRequestID, resp.Header.Get("X-GitHub-Request-Id"),
		logging.KeyRateRemaining, resp.Header.Get("X-RateLimit-Remaining"))...)
	return resp, nil
}

type etagKey struct{}

// withETag has requests made with the returned context ask for what changed
// since etag. The REST client has no other way to set a request's header.
func withETag(ctx context.Context, etag string) context.Context {
	return context.WithValue(ctx, etagKey{}, etag)
}
