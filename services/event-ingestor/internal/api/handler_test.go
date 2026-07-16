package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"eventmesh/event-ingestor/internal/auth"
	"eventmesh/event-ingestor/internal/idempotency"
	"eventmesh/event-ingestor/internal/model"
	"eventmesh/event-ingestor/internal/producer"
	"eventmesh/event-ingestor/internal/ratelimit"
	"eventmesh/pkg/metrics"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
)

// MockAuthServer mock
type MockAuthServer struct {
	server *httptest.Server
	keys   map[string]string
}

func NewMockAuthServer() *MockAuthServer {
	s := &MockAuthServer{
		keys: make(map[string]string),
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		tenantID, ok := s.keys[key]
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"tenant_id": tenantID})
	}))
	return s
}

func (m *MockAuthServer) Close() {
	m.server.Close()
}

// MockAsyncProducer mock
type MockAsyncProducer struct {
	input  chan *sarama.ProducerMessage
	errors chan *sarama.ProducerError
}

func (m *MockAsyncProducer) AsyncClose() {}
func (m *MockAsyncProducer) Close() error {
	close(m.input)
	close(m.errors)
	return nil
}
func (m *MockAsyncProducer) Input() chan<- *sarama.ProducerMessage     { return m.input }
func (m *MockAsyncProducer) Successes() <-chan *sarama.ProducerMessage { return nil }
func (m *MockAsyncProducer) Errors() <-chan *sarama.ProducerError      { return m.errors }
func (m *MockAsyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag  { return 0 }
func (m *MockAsyncProducer) IsTransactional() bool                     { return false }
func (m *MockAsyncProducer) BeginTxn() error                           { return nil }
func (m *MockAsyncProducer) CommitTxn() error                          { return nil }
func (m *MockAsyncProducer) AbortTxn() error                           { return nil }
func (m *MockAsyncProducer) AddOffsetsToTxn(offsets map[string][]*sarama.PartitionOffsetMetadata, groupId string) error {
	return nil
}
func (m *MockAsyncProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupId string, metadata *string) error {
	return nil
}

func TestHandler_IngestEvent(t *testing.T) {
	// Initialize metrics
	metrics.Init()

	// 1. Setup mock auth server
	authServer := NewMockAuthServer()
	defer authServer.Close()
	authServer.keys["valid-key"] = "tenant-1"

	authClient := auth.NewClient(authServer.server.URL)
	jwtValidator := auth.NewJWTValidator()

	// 2. Setup mock Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6380",
	})
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		redisClient = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		_, err = redisClient.Ping(ctx).Result()
		if err != nil {
			t.Skip("Redis is not running, skipping HTTP handler test")
		}
	}

	idempotencyStore := idempotency.NewStore(redisClient.Options().Addr, 5*time.Minute)

	// 3. Setup mock producer
	mockAsync := &MockAsyncProducer{
		input:  make(chan *sarama.ProducerMessage, 100),
		errors: make(chan *sarama.ProducerError, 100),
	}
	eventProducer := producer.NewProducerWithAsyncProducer(mockAsync, "events")

	rateLimiter := ratelimit.NewLimiter(redisClient)

	handler := NewHandler(authClient, jwtValidator, idempotencyStore, rateLimiter, eventProducer)

	// Clean up Redis test keys
	testKey := "test-idemp-1"
	redisClient.Del(ctx, testKey)
	defer redisClient.Del(ctx, testKey)

	// Case 1: Ingest with valid API key (Success 202)
	payload := map[string]interface{}{"order_id": "123"}
	reqBody, _ := json.Marshal(model.IngestEventRequest{
		EventType: "order.placed",
		Payload:   payload,
	})

	req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(reqBody))
	req.Header.Set("X-API-Key", "valid-key")
	req.Header.Set("Idempotency-Key", testKey)

	w := httptest.NewRecorder()
	handler.IngestEvent(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 Accepted, got: %d", w.Code)
	}

	// Case 2: Duplicate Ingestion (Should return 200 status "duplicate")
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/events", bytes.NewBuffer(reqBody))
	req2.Header.Set("X-API-Key", "valid-key")
	req2.Header.Set("Idempotency-Key", testKey) // Same key

	handler.IngestEvent(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK for duplicate, got: %d", w2.Code)
	}

	var resp struct {
		Status string `json:"status"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp.Status != "duplicate" {
		t.Errorf("Expected status 'duplicate', got: %s", resp.Status)
	}

	// Case 3: Unauthorized Ingestion (Should return 401)
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("POST", "/events", bytes.NewBuffer(reqBody))
	req3.Header.Set("X-API-Key", "invalid-key")
	req3.Header.Set("Idempotency-Key", "another-key")

	handler.IngestEvent(w3, req3)

	if w3.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got: %d", w3.Code)
	}
}
