package plugin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// The raw JSON-RPC /mcp route bypassed RBAC, tool selection, Graphiti org
// scoping and the approval gate. It must stay unregistered.
func TestRawMCPRouteIsNotRegistered(t *testing.T) {
	p := &Plugin{logger: log.DefaultLogger}
	mux := http.NewServeMux()
	p.registerRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"any_tool","arguments":{}}}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// The catch-all "/" handler may answer; the request must NOT reach the MCP
	// proxy, i.e. the response must be the default page, not a JSON-RPC result.
	if !strings.Contains(rec.Body.String(), "Consensys Assistant Backend Plugin") {
		t.Fatalf("/mcp must fall through to the default handler, got: %s", rec.Body.String())
	}
}

// Client-supplied X-Grafana-* identity headers must not influence the
// resolved role; without a server-populated plugin context the role is Viewer.
func TestGetUserRoleIgnoresHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Grafana-User-Role", "Admin")
	req.Header.Set("X-Grafana-Org-Role", "Admin")
	req.Header.Set("X-Grafana-Role", "Admin")

	if got := getUserRole(req); got != "Viewer" {
		t.Fatalf("getUserRole = %q, want Viewer (headers must be ignored)", got)
	}
}

// Client-supplied X-Grafana-User-Id must not forge another user's identity.
func TestGetUserIDIgnoresHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Grafana-User-Id", "42")

	if got := getUserID(req); got != 0 {
		t.Fatalf("getUserID = %d, want 0 (header must be ignored)", got)
	}
}

// Org scoping must come from the server-populated plugin context, not from
// the client-controllable X-Grafana-Org-Id header.
func TestGetOrgIDPrefersPluginContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Grafana-Org-Id", "99")
	if got := getOrgID(req); got != 1 {
		t.Fatalf("getOrgID without context = %d, want 1 (header must be ignored)", got)
	}

	req = withTestIdentity(req, 5, "user-7", "Viewer")
	if got := getOrgID(req); got != 5 {
		t.Fatalf("getOrgID with context = %d, want 5", got)
	}
}

func TestHandleGraphitiIngestSessionRequiresEditorOrAdmin(t *testing.T) {
	p := &Plugin{logger: log.DefaultLogger}

	req := httptest.NewRequest(http.MethodPost, "/api/graphiti/ingest-session", strings.NewReader(`{"messages":[{"role":"user","content":"inject"}]}`))
	req = withTestIdentity(req, 1, "user-viewer", "Viewer")
	rec := httptest.NewRecorder()

	p.handleGraphitiIngestSession(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for Viewer ingest", rec.Code)
	}
}

func TestHandleGraphitiDiscoverRequiresEditorOrAdmin(t *testing.T) {
	p := &Plugin{logger: log.DefaultLogger}

	req := httptest.NewRequest(http.MethodPost, "/api/graphiti/discover", nil)
	req = withTestIdentity(req, 1, "user-viewer", "Viewer")
	rec := httptest.NewRecorder()

	p.handleGraphitiDiscover(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for Viewer discover", rec.Code)
	}
}

func TestHandleAgentRunIsRateLimited(t *testing.T) {
	p := newAgentRunTestPlugin(t)
	p.agentRateLimiter = NewScopedInMemoryRateLimiter(log.DefaultLogger, 1, ShareRateLimitWindow)

	allowed := withTestIdentity(httptest.NewRequest(http.MethodPost, "/api/agent/run", strings.NewReader(`{"message":"hi"}`)), 1, "user-7", "Admin")
	// First request consumes the single token; it should get past the limiter
	// (it may still fail later on missing Grafana config, but not with 429).
	rec := httptest.NewRecorder()
	p.handleAgentRun(rec, allowed)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("first request must not be rate limited")
	}

	limited := withTestIdentity(httptest.NewRequest(http.MethodPost, "/api/agent/run", strings.NewReader(`{"message":"hi"}`)), 1, "user-7", "Admin")
	rec = httptest.NewRecorder()
	p.handleAgentRun(rec, limited)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec.Code)
	}
}

func TestHandleMCPCallToolIsRateLimited(t *testing.T) {
	p := newAgentRunTestPlugin(t)
	p.toolCallRateLimiter = NewScopedInMemoryRateLimiter(log.DefaultLogger, 1, ShareRateLimitWindow)

	mk := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/mcp/call-tool", strings.NewReader(`{"name":"any_tool","arguments":{}}`))
		req = withTestIdentity(req, 1, "user-7", "Admin")
		rec := httptest.NewRecorder()
		p.handleMCPCallTool(rec, req)
		return rec
	}

	if rec := mk(); rec.Code == http.StatusTooManyRequests {
		t.Fatalf("first request must not be rate limited")
	}
	if rec := mk(); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec.Code)
	}
}

// The in-memory limiter map must stay bounded even under a flood of distinct
// user IDs (previously it grew without limit).
func TestInMemoryRateLimiterBoundsTrackedUsers(t *testing.T) {
	rl := NewScopedInMemoryRateLimiter(log.DefaultLogger, 100, ShareRateLimitWindow)
	for i := int64(0); i < maxTrackedUsers+10; i++ {
		rl.CheckLimit(i)
	}
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	if len(rl.limiters) > maxTrackedUsers {
		t.Fatalf("tracked users = %d, want <= %d", len(rl.limiters), maxTrackedUsers)
	}
}
