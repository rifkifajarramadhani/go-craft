package elasticsearchadapter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"__GO_CRAFT_MODULE__/internal/config"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
)

func TestOpenChecksClusterAndSendsBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "elastic" || password != "secret" {
			t.Errorf("basic auth = %q/%q/%t", username, password, ok)
		}

		writer.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = io.WriteString(writer, `{"version":{"number":"8.19.18"}}`)
	}))
	defer server.Close()

	client, err := Open(context.Background(), config.ElasticsearchConfig{
		URL: server.URL, Username: "elastic", Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	if client == nil {
		t.Fatal("expected client")
	}
}

func TestOpenReturnsHTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Elastic-Product", "Elasticsearch")
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	_, err := Open(context.Background(), config.ElasticsearchConfig{URL: server.URL})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("error = %v, want HTTP 400", err)
	}
}

func TestOpenRetriesTransientStatuses(t *testing.T) {
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Elastic-Product", "Elasticsearch")

		if requests.Add(1) < 3 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()

	if _, err := Open(context.Background(), config.ElasticsearchConfig{URL: server.URL}); err != nil {
		t.Fatal(err)
	}

	if requests.Load() != 3 {
		t.Fatalf("requests = %d, want 3", requests.Load())
	}
}

func TestCheckClosesResponseBody(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader(`{}`)}

	client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
			Body:       body,
		}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}

	if err := Check(context.Background(), client); err != nil {
		t.Fatal(err)
	}

	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestCheckHonorsContextCancellation(t *testing.T) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Check(ctx, client); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestCheckRecoversAfterClusterReturns(t *testing.T) {
	var healthy atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Elastic-Product", "Elasticsearch")

		if !healthy.Load() {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()

	client, err := newClient(config.ElasticsearchConfig{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	if err := Check(context.Background(), client); err == nil {
		t.Fatal("expected unavailable cluster error")
	}

	healthy.Store(true)

	if err := Check(context.Background(), client); err != nil {
		t.Fatalf("check after recovery: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (body *trackingBody) Close() error {
	body.closed = true
	return nil
}
