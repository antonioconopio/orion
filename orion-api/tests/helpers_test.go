// Package tests holds black-box tests for the Orion API: HTTP endpoint tests
// and the Redis queue (unit-level and end to end).
//
// They need the docker-compose Postgres and Redis running. They never touch
// the dev database: a separate "<db>_test" database is created on the same
// server, and Redis tests use uniquely named streams that are deleted after.
//
// Overrides: TEST_DATABASE_URL, TEST_REDIS_ADDR (default localhost:6379).
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"orion-api/internal/db"
	"orion-api/internal/handlers"
)

const migrationPath = "../../migrations/000001_init_schema.up.sql"

var (
	poolOnce sync.Once
	testPool *pgxpool.Pool
	poolErr  error
)

func redisAddr() string {
	if a := os.Getenv("TEST_REDIS_ADDR"); a != "" {
		return a
	}
	return "localhost:6379"
}

// testDatabaseURL returns the URL of the throwaway test database.
func testDatabaseURL() (testURL, adminURL string, err error) {
	godotenv.Load("../.env")
	adminURL = os.Getenv("DATABASE_URL")
	testURL = os.Getenv("TEST_DATABASE_URL")
	if testURL == "" {
		if adminURL == "" {
			return "", "", fmt.Errorf("DATABASE_URL (or TEST_DATABASE_URL) is not set")
		}
		u, err := url.Parse(adminURL)
		if err != nil {
			return "", "", err
		}
		u.Path = "/" + strings.TrimPrefix(u.Path, "/") + "_test"
		testURL = u.String()
	}
	return testURL, adminURL, nil
}

// sharedPool lazily creates the test DB (if missing), applies the schema
// (if missing) and returns a pool. Skips tests if Postgres is unreachable.
func sharedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	poolOnce.Do(func() { testPool, poolErr = initPool() })
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	return testPool
}

func initPool() (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	testURL, adminURL, err := testDatabaseURL()
	if err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig(testURL)
	if err != nil {
		return nil, err
	}
	// Safety net: these tests TRUNCATE everything.
	if !strings.HasSuffix(cfg.Database, "_test") {
		return nil, fmt.Errorf("refusing to run against database %q: name must end in _test", cfg.Database)
	}

	if pool, err := db.NewPool(ctx, testURL); err == nil {
		return ensureSchema(ctx, pool)
	}

	// Test DB doesn't exist yet: create it via the admin connection.
	if adminURL == "" {
		return nil, fmt.Errorf("cannot connect to %s and no DATABASE_URL to create it", cfg.Database)
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return nil, err
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q`, cfg.Database)); err != nil {
		return nil, err
	}
	pool, err := db.NewPool(ctx, testURL)
	if err != nil {
		return nil, err
	}
	return ensureSchema(ctx, pool)
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) (*pgxpool.Pool, error) {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.dags') IS NOT NULL`).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return pool, nil
	}
	sql, err := os.ReadFile(migrationPath)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		return nil, err
	}
	return pool, nil
}

// apiClient is a tiny JSON client against an httptest server running the
// real routing table.
type apiClient struct {
	t    *testing.T
	srv  *httptest.Server
	pool *pgxpool.Pool
}

// newAPI starts the API on a clean database.
func newAPI(t *testing.T) *apiClient {
	t.Helper()
	pool := sharedPool(t)
	if _, err := pool.Exec(context.Background(), `TRUNCATE dags, tasks, task_dependencies, runs, task_instances CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, pool)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &apiClient{t: t, srv: srv, pool: pool}
}

// do sends a request. body may be nil, a string (sent raw) or any
// JSON-marshalable value. It returns the status and raw response body.
func (c *apiClient) do(method, path string, body any) (int, []byte) {
	c.t.Helper()
	var rdr *bytes.Reader
	switch b := body.(type) {
	case nil:
		rdr = bytes.NewReader(nil)
	case string:
		rdr = bytes.NewReader([]byte(b))
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			c.t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.srv.URL+path, rdr)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

// call sends a request, asserts the status, and decodes the JSON response
// into out (if non-nil).
func (c *apiClient) call(method, path string, body any, wantStatus int, out any) {
	c.t.Helper()
	status, raw := c.do(method, path, body)
	if status != wantStatus {
		c.t.Fatalf("%s %s: status = %d, want %d (body: %s)", method, path, status, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decoding %q: %v", method, path, raw, err)
		}
	}
}

func (c *apiClient) expectStatus(method, path string, body any, wantStatus int) {
	c.t.Helper()
	c.call(method, path, body, wantStatus, nil)
}

// Minimal response shapes (decoupled from internal/models on purpose, so
// tests also catch accidental JSON-contract changes).
type dagResp struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Schedule    *string `json:"schedule"`
	Owner       *string `json:"owner"`
	IsActive    bool    `json:"is_active"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type taskResp struct {
	ID       string          `json:"id"`
	DAGID    string          `json:"dag_id"`
	Name     string          `json:"name"`
	TaskType string          `json:"task_type"`
	Config   json.RawMessage `json:"config"`
}

type runResp struct {
	ID     string `json:"id"`
	DAGID  string `json:"dag_id"`
	Status string `json:"status"`
}

type taskInstanceResp struct {
	ID     string `json:"id"`
	RunID  string `json:"run_id"`
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func (c *apiClient) createDAG(name string) dagResp {
	c.t.Helper()
	var d dagResp
	c.call("POST", "/dags", map[string]any{"name": name}, http.StatusCreated, &d)
	return d
}

func (c *apiClient) createTask(dagID, name string) taskResp {
	c.t.Helper()
	var tk taskResp
	c.call("POST", "/dags/"+dagID+"/tasks", map[string]any{
		"name": name, "task_type": "shell", "config": map[string]any{"cmd": "echo " + name},
	}, http.StatusCreated, &tk)
	return tk
}

func (c *apiClient) triggerRun(dagID string) runResp {
	c.t.Helper()
	var r runResp
	c.call("POST", "/dags/"+dagID+"/runs", nil, http.StatusCreated, &r)
	return r
}

func randomUUID() string { return uuid.NewString() }

// --- Redis helpers ---

// redisClient returns a raw client for assertions, skipping if Redis is down.
func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	rc := redis.NewClient(&redis.Options{Addr: redisAddr()})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rc.Ping(ctx).Err(); err != nil {
		rc.Close()
		t.Skipf("redis unavailable at %s: %v", redisAddr(), err)
	}
	t.Cleanup(func() { rc.Close() })
	return rc
}

// uniqueStreamName returns a stream name private to this test and deletes
// the stream when the test ends.
func uniqueStreamName(t *testing.T, rc *redis.Client) string {
	t.Helper()
	name := "orion:test:" + uuid.NewString()
	t.Cleanup(func() { rc.Del(context.Background(), name) })
	return name
}
