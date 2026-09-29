// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/opensearch-project/opensearch-go/v4"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	backendElasticsearch = model.ElasticsearchSettingsESBackend
	backendOpenSearch    = model.ElasticsearchSettingsOSBackend

	sniffInterval = 5 * time.Minute
)

// performer is the minimal surface we need from the official clients: send
// a fully built HTTP request to one of the cluster nodes, with retries, node
// selection, authentication, etc. handled by the client's transport.
type performer func(req *http.Request) (*http.Response, error)

// client is a thin, backend agnostic REST client. Elasticsearch and
// OpenSearch share the subset of the REST API we use (documents, bulk, search,
// by-query operations, index templates, aliases, tasks), so everything above
// the transport is common code.
type client struct {
	backend string
	perform performer
	closeFn func()
	timeout time.Duration
}

// apiError is a non 2xx answer from the search backend.
type apiError struct {
	Status int
	Type   string
	Reason string
	Body   string
}

func (e *apiError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("search backend returned status %d: %s: %s", e.Status, e.Type, e.Reason)
	}
	body := e.Body
	if len(body) > 512 {
		body = body[:512] + "..."
	}
	return fmt.Sprintf("search backend returned status %d: %s", e.Status, body)
}

func isNotFound(err error) bool {
	var ae *apiError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

func isErrorType(err error, errType string) bool {
	var ae *apiError
	return errors.As(err, &ae) && ae.Type == errType
}

// clientFileReader reads a certificate file referenced by the configuration.
type clientFileReader func(name string) ([]byte, error)

func readConfigFile(name string, fallback clientFileReader) ([]byte, error) {
	data, err := os.ReadFile(name)
	if err == nil {
		return data, nil
	}
	if fallback != nil {
		if fbData, fbErr := fallback(name); fbErr == nil {
			return fbData, nil
		}
	}
	return nil, err
}

// newClient builds a client for the given settings.
func newClient(settings *model.ElasticsearchSettings, logger mlog.LoggerIFace, readFile clientFileReader) (*client, *model.AppError) {
	backend := backendName(settings)
	params := map[string]any{"Backend": backendDisplayName(backend)}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: model.SafeDereference(settings.SkipTLSVerification), //nolint:gosec // explicitly requested by the administrator
	}

	if ca := model.SafeDereference(settings.CA); ca != "" {
		pem, err := readConfigFile(ca, readFile)
		if err != nil {
			return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.ca_cert_missing", params, "", http.StatusInternalServerError).Wrap(err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.ca_cert_missing", params, "no certificate found in the CA file", http.StatusInternalServerError)
		}
		tlsConfig.RootCAs = pool
	}

	certFile := model.SafeDereference(settings.ClientCert)
	keyFile := model.SafeDereference(settings.ClientKey)
	if certFile != "" || keyFile != "" {
		certPEM, err := readConfigFile(certFile, readFile)
		if err != nil {
			return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.client_cert_missing", params, "", http.StatusInternalServerError).Wrap(err)
		}
		keyPEM, err := readConfigFile(keyFile, readFile)
		if err != nil {
			return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.client_key_missing", params, "", http.StatusInternalServerError).Wrap(err)
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.client_cert_malformed", params, "", http.StatusInternalServerError).Wrap(err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.TLSClientConfig = tlsConfig
	httpTransport.MaxIdleConnsPerHost = 20

	var roundTripper http.RoundTripper = httpTransport
	if trace := model.SafeDereference(settings.Trace); trace == "all" || trace == "error" {
		roundTripper = &tracingTransport{next: httpTransport, logger: logger, all: trace == "all"}
	}

	var addresses []string
	for addr := range strings.SplitSeq(model.SafeDereference(settings.ConnectionURL), ",") {
		if addr = strings.TrimSpace(addr); addr != "" {
			addresses = append(addresses, addr)
		}
	}
	if len(addresses) == 0 {
		return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.connect_failed", params, "no connection URL configured", http.StatusInternalServerError)
	}

	sniff := model.SafeDereference(settings.Sniff)
	var discoverInterval time.Duration
	if sniff {
		discoverInterval = sniffInterval
	}

	c := &client{
		backend: backend,
		timeout: time.Duration(max(model.SafeDereference(settings.RequestTimeoutSeconds), 1)) * time.Second,
	}

	switch backend {
	case backendOpenSearch:
		osClient, err := opensearch.NewClient(opensearch.Config{
			Addresses:             addresses,
			Username:              model.SafeDereference(settings.Username),
			Password:              model.SafeDereference(settings.Password),
			Transport:             roundTripper,
			DiscoverNodesOnStart:  model.NewPointer(sniff),
			DiscoverNodesInterval: discoverInterval,
		})
		if err != nil {
			return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.connect_failed", params, "", http.StatusInternalServerError).Wrap(err)
		}
		c.perform = osClient.Stream
		c.closeFn = func() { _ = osClient.Close() }
	default:
		esClient, err := elasticsearch.NewClient(elasticsearch.Config{
			Addresses:             addresses,
			Username:              model.SafeDereference(settings.Username),
			Password:              model.SafeDereference(settings.Password),
			Transport:             roundTripper,
			DiscoverNodesOnStart:  sniff,
			DiscoverNodesInterval: discoverInterval,
		})
		if err != nil {
			return nil, model.NewAppError("newClient", "ent.elasticsearch.create_client.connect_failed", params, "", http.StatusInternalServerError).Wrap(err)
		}
		c.perform = esClient.Perform
		c.closeFn = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = esClient.Close(ctx)
		}
	}

	return c, nil
}

func (c *client) close() {
	if c != nil && c.closeFn != nil {
		c.closeFn()
	}
}

// do executes a request. body may be nil, a []byte (sent as is) or any value
// that is JSON encoded. It returns the raw response body of 2xx answers and an
// *apiError otherwise.
func (c *client) do(ctx context.Context, method, path string, query url.Values, body any) ([]byte, error) {
	var reader io.Reader
	contentType := "application/json"
	switch b := body.(type) {
	case nil:
	case []byte:
		reader = bytes.NewReader(b)
		if strings.HasSuffix(path, "/_bulk") {
			contentType = "application/x-ndjson"
		}
	default:
		encoded, err := json.Marshal(b)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	if reader != nil {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.perform(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return data, nil
	}
	return nil, parseAPIError(res.StatusCode, data)
}

func parseAPIError(status int, data []byte) *apiError {
	ae := &apiError{Status: status, Body: string(data)}
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &payload) == nil && len(payload.Error) > 0 {
		var detailed struct {
			Type      string `json:"type"`
			Reason    string `json:"reason"`
			RootCause []struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"root_cause"`
		}
		if json.Unmarshal(payload.Error, &detailed) == nil {
			ae.Type = detailed.Type
			ae.Reason = detailed.Reason
			if ae.Reason == "" && len(detailed.RootCause) > 0 {
				ae.Reason = detailed.RootCause[0].Reason
			}
		} else {
			var plain string
			if json.Unmarshal(payload.Error, &plain) == nil {
				ae.Reason = plain
			}
		}
	}
	return ae
}

// doJSON executes a request and decodes the answer into out (if not nil).
func (c *client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	data, err := c.do(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to decode search backend response: %w", err)
	}
	return nil
}

// serverInfo is the answer of "GET /".
type serverInfo struct {
	Version struct {
		Number       string `json:"number"`
		Distribution string `json:"distribution"`
	} `json:"version"`
	Tagline string `json:"tagline"`
}

func (c *client) info(ctx context.Context) (*serverInfo, error) {
	var info serverInfo
	if err := c.doJSON(ctx, http.MethodGet, "/", nil, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *client) plugins(ctx context.Context) ([]string, error) {
	var rows []struct {
		Component string `json:"component"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/_cat/plugins", url.Values{"format": {"json"}}, nil, &rows); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	plugins := []string{}
	for _, row := range rows {
		if row.Component != "" && !seen[row.Component] {
			seen[row.Component] = true
			plugins = append(plugins, row.Component)
		}
	}
	return plugins, nil
}

// tracingTransport logs requests sent to the search backend, as requested by
// the ElasticsearchSettings.Trace setting.
type tracingTransport struct {
	next   http.RoundTripper
	logger mlog.LoggerIFace
	all    bool
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	res, err := t.next.RoundTrip(req)
	fields := []mlog.Field{
		mlog.String("method", req.Method),
		mlog.String("url", redactURL(req.URL)),
		mlog.Duration("duration", time.Since(start)),
	}
	switch {
	case err != nil:
		t.logger.Warn("Search backend request failed", append(fields, mlog.Err(err))...)
	case res.StatusCode >= 400:
		t.logger.Warn("Search backend request returned an error", append(fields, mlog.Int("status", res.StatusCode))...)
	case t.all:
		t.logger.Info("Search backend request", append(fields, mlog.Int("status", res.StatusCode))...)
	}
	return res, err
}

func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	clone := *u
	clone.User = nil
	return clone.String()
}
