package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// preCRBaselineBytes is the documented pre-CR-0060 cold-start schema baseline
// byte count. It was estimated by serialising the 32 individual tool definitions
// (14 calendar + 11 mail + 6 account + 1 system + complete_auth, all feature
// flags enabled) that the server registered before CR-0060 was applied.
//
// The estimate is derived in docs/cr/CR-0060-validation-report.md. It is
// hardcoded here because the pre-CR tool surface no longer exists in the
// codebase, per CR-0060 Phase 3's clean-cutover approach.
const preCRBaselineBytes = 74_000 // conservative lower-bound estimate in bytes

// minRequiredReductionPct is the minimum percentage reduction in tool-schema
// byte count required by CR-0060 NFR-1 and AC-8.
const minRequiredReductionPct = 60

// TestColdStartSchemaSize_Reduction verifies that the cold-start schema byte
// count of the four aggregate tools is at least 60% smaller than the pre-CR
// baseline (CR-0060 AC-8 / NFR-1).
//
// The test registers all four domain tools with all feature flags enabled
// (the maximum-size configuration), serialises each tool's definition to JSON,
// sums the byte counts, and asserts the reduction exceeds the threshold.
func TestColdStartSchemaSize_Reduction(t *testing.T) {
	s := mcpserver.NewMCPServer("test-schema-size", "0.0.0",
		mcpserver.WithToolCapabilities(false),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	r := auth.NewAccountRegistry()
	_ = r.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	audit.InitAuditLog(false, "")

	// Use maximum feature-flag configuration to get the largest possible
	// post-CR schema (all mail verbs enabled).
	cfg := config.Config{
		AuthRecordPath:    "/tmp/test-schema",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	}

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, r, cfg, nil)

	registered := s.ListTools()

	// Sum serialised byte count of all registered tool definitions.
	postCRBytes := 0
	for _, st := range registered {
		data, err := json.Marshal(st.Tool)
		if err != nil {
			t.Fatalf("json.Marshal tool %q: %v", st.Tool.Name, err)
		}
		postCRBytes += len(data)
	}

	reductionPct := 100 * (preCRBaselineBytes - postCRBytes) / preCRBaselineBytes

	t.Logf("pre-CR baseline: %d bytes (documented estimate)", preCRBaselineBytes)
	t.Logf("post-CR schema:  %d bytes (%d tools)", postCRBytes, len(registered))
	t.Logf("reduction:       %d%% (required >= %d%%)", reductionPct, minRequiredReductionPct)

	if reductionPct < minRequiredReductionPct {
		t.Errorf("schema size reduction is %d%%, want >= %d%% (pre=%d bytes, post=%d bytes)",
			reductionPct, minRequiredReductionPct, preCRBaselineBytes, postCRBytes)
	}
}
