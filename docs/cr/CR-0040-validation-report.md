# CR-0040 Validation Report

**CR**: CR-0040 -- MCP Event Provenance Tagging
**Validated**: 2026-03-19
**Branch**: dev/cc-swarm
**Commit**: 33d6026

## Summary

Requirements: 10/10 | Acceptance Criteria: 10/10 | Tests: 19/19 | Gaps: 0 (4 fixed)

All functional requirements and acceptance criteria are fully implemented and traceable
to source code. All 19 specified tests exist and match their specifications. Four
previously missing tool-level integration tests have been added (gap fix pass).

Build: PASS | Lint: PASS (0 issues) | Tests: PASS (all packages)

## Requirement Verification

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| FR-1 | Every create_event MUST include provenance extended property | PASS | `internal/tools/create_event.go:317-323` -- stamps property when `provenancePropertyID != ""` |
| FR-2 | Provenance tag configurable via OUTLOOK_MCP_PROVENANCE_TAG with default | PASS | `internal/config/config.go:259-263` -- uses `os.LookupEnv` with default `com.github.desek.outlook-local-mcp.created` |
| FR-3 | Empty tag disables provenance entirely | PASS | Config: `config.go:259` sets empty. Server: `server.go:66-68` only builds ID when tag non-empty. Handlers: all guard on `provenancePropertyID != ""`. Tool def: `server.go:81` passes `provenancePropertyID != ""` to `NewSearchEventsTool`. |
| FR-4 | Property ID constructed once at startup, not per request | PASS | `internal/server/server.go:66-68` -- `BuildProvenancePropertyID` called once, result passed to all handlers |
| FR-5 | get_event MUST $expand provenance and include createdByMcp | PASS | `internal/tools/get_event.go:124-126` ($expand), `get_event.go:173,176` (passes provenancePropertyID to serializers) |
| FR-6 | list_events MUST $expand provenance and include createdByMcp | PASS | `internal/tools/list_events.go:193-195` ($expand), `list_events.go:295,297` (passes provenancePropertyID to serializers) |
| FR-7 | search_events MUST $expand provenance and include createdByMcp | PASS | `internal/tools/search_events.go:287-289` ($expand), `search_events.go:294` (passes provenancePropertyID to serializer) |
| FR-8 | search_events MUST accept optional created_by_mcp boolean parameter | PASS | `internal/tools/search_events.go:107-111` (param registration), `search_events.go:222-225,268-273` (OData filter) |
| FR-9 | Provenance MUST NOT be visible in Outlook UI | PASS | Uses single-value extended properties (MAPI named properties), which are invisible in the Outlook UI by design. No custom UI fields are created. |
| FR-10 | Provenance MUST NOT be set on update_event | PASS | `internal/tools/update_event.go` -- no references to provenance, SingleValueExtendedProperties, or singleValueExtendedProperties anywhere in the file |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Event tagged on creation | PASS | `create_event.go:317-323` sets extended property with provenance ID and value "true". Test: `TestCreateEvent_SetsProvenanceProperty` verifies POST body contains `singleValueExtendedProperties` and GUID. |
| AC-2 | Tag visible on get_event | PASS | `get_event.go:124-126` adds `$expand`. `get_event.go:173,176` passes propertyID to serializers. `SerializeSummaryGetEvent` delegates to `SerializeSummaryEvent` which checks `HasProvenanceTag`. Test: `TestSerializeSummaryGetEvent_CreatedByMcp` in `serialize_test.go:520-541`. |
| AC-3 | Tag visible on list_events | PASS | `list_events.go:193-195` adds `$expand`. `list_events.go:295,297` passes propertyID to serializers. Test: `TestSerializeSummaryEvent_CreatedByMcp` in `serialize_test.go:494-515`. |
| AC-4 | Tag absent for non-MCP events | PASS | `serialize.go:98-102` only sets `createdByMcp` when `HasProvenanceTag` returns true; omits key entirely otherwise. Tests: `TestSerializeEvent_NoMcpTag` (`serialize_test.go:478-490`), `TestHasProvenanceTag_Absent` (`provenance_test.go:65-73`), `TestHasProvenanceTag_OtherProperty` (`provenance_test.go:77-91`). |
| AC-5 | Search filter by provenance | PASS | `search_events.go:268-273` appends `singleValueExtendedProperties/Any(...)` OData filter when `createdByMcp && provenancePropertyID != ""`. Test: `TestSearchEvents_CreatedByMcpFilter` (`search_events_test.go:459-494`). |
| AC-6 | Custom provenance tag name | PASS | `config.go:259` reads from env var. `BuildProvenancePropertyID` formats with any tag name. Tests: `TestLoadConfig_ProvenanceTagCustom` (`config_test.go:847-857`), `TestBuildProvenancePropertyID_CustomTag` (`provenance_test.go:22-29`). |
| AC-7 | Provenance tagging disabled | PASS | `config.go:259-263` returns empty string when env var is explicitly empty. `server.go:66-68` skips `BuildProvenancePropertyID` when empty. All handlers guard on `provenancePropertyID != ""`. `NewSearchEventsTool(false)` omits `created_by_mcp` param. Tests: `TestLoadConfig_ProvenanceTagEmpty` (`config_test.go:862-871`), `TestCreateEvent_ProvenanceDisabled` (`create_event_test.go:270-294`), `TestSearchEvents_CreatedByMcpParam` (`search_events_test.go:533-543`), `TestSearchEvents_CreatedByMcpFilterDisabled` (`search_events_test.go:498-528`). |
| AC-8 | No UI visibility | PASS | Uses MAPI single-value extended properties, which are invisible in Outlook by design. This is a Graph API platform guarantee, not verifiable by unit test. |
| AC-9 | Tag visible on search_events | PASS | `search_events.go:287-289` adds `$expand`. `search_events.go:294` passes propertyID to serializer. Summary conversion via `ToSummaryEventMap` propagates `createdByMcp` (`serialize.go:166-168`). Test: `TestSerializeEvent_CreatedByMcp` (`serialize_test.go:450-473`) covers the raw serialization path used by search_events. |
| AC-10 | Provenance not set on update_event | PASS | `update_event.go` contains zero references to provenance, extended properties, or `SingleValueExtendedProperties`. The PATCH body construction does not include any extended property logic. |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| `provenance_test.go` | `TestBuildProvenancePropertyID` | Yes | Yes | Yes |
| `provenance_test.go` | `TestBuildProvenancePropertyID_CustomTag` | Yes | Yes | Yes |
| `provenance_test.go` | `TestNewProvenanceProperty` | Yes | Yes | Yes |
| `provenance_test.go` | `TestHasProvenanceTag_Present` | Yes | Yes | Yes |
| `provenance_test.go` | `TestHasProvenanceTag_Absent` | Yes | Yes | Yes |
| `provenance_test.go` | `TestHasProvenanceTag_OtherProperty` | Yes | Yes | Yes |
| `config_test.go` | `TestLoadConfig_ProvenanceTagDefault` | Yes | Yes | Yes |
| `config_test.go` | `TestLoadConfig_ProvenanceTagCustom` | Yes | Yes | Yes |
| `config_test.go` | `TestLoadConfig_ProvenanceTagEmpty` | Yes | Yes | Yes |
| `serialize_test.go` | `TestSerializeEvent_CreatedByMcp` | Yes | Yes | Yes |
| `serialize_test.go` | `TestSerializeEvent_NoMcpTag` | Yes | Yes | Yes |
| `create_event_test.go` | `TestCreateEvent_SetsProvenanceProperty` | Yes | Yes | Yes |
| `search_events_test.go` | `TestSearchEvents_CreatedByMcpFilter` | Yes | Yes | Yes |
| `create_event_test.go` | `TestCreateEvent_ProvenanceDisabled` | Yes | Yes | Yes |
| `get_event_test.go` | `TestGetEvent_CreatedByMcpInResponse` | Yes | Yes | Yes |
| `list_events_test.go` | `TestListEvents_CreatedByMcpInResponse` | Yes | Yes | Yes |
| `search_events_test.go` | `TestSearchEvents_CreatedByMcpInResponse` | Yes | Yes | Yes |
| `update_event_test.go` | `TestUpdateEvent_NoProvenanceProperty` | Yes | Yes | Yes |

### Additional tests found (not in spec, but relevant)

| Test File | Test Name | Coverage |
|-----------|-----------|----------|
| `provenance_test.go` | `TestProvenanceExpandFilter` | Validates $expand filter string construction |
| `serialize_test.go` | `TestSerializeSummaryEvent_CreatedByMcp` | Validates createdByMcp in summary format (covers list_events path) |
| `serialize_test.go` | `TestSerializeSummaryGetEvent_CreatedByMcp` | Validates createdByMcp in get-event summary format |
| `serialize_test.go` | `TestSerializeEvent_NoProvenanceID` | Validates createdByMcp omitted when provenance disabled |
| `search_events_test.go` | `TestSearchEvents_CreatedByMcpParam` | Validates parameter presence/absence based on provenance flag |
| `search_events_test.go` | `TestSearchEvents_CreatedByMcpFilterDisabled` | Validates filter ignored when provenance disabled |

## Gaps

All gaps resolved. Original 4 gaps fixed in gap-fix pass:

1. **`TestGetEvent_CreatedByMcpInResponse`** (get_event_test.go:548) -- FIXED. Handler-level test calls get_event with mock Graph response containing provenance extended property, asserts `createdByMcp: true` in JSON output.

2. **`TestListEvents_CreatedByMcpInResponse`** (list_events_test.go:494) -- FIXED. Handler-level test calls list_events with mock Graph response containing provenance extended property, asserts `createdByMcp: true` in serialized output.

3. **`TestSearchEvents_CreatedByMcpInResponse`** (search_events_test.go:548) -- FIXED. Handler-level test calls search_events with mock Graph response containing provenance extended property, asserts `createdByMcp: true` in serialized output.

4. **`TestUpdateEvent_NoProvenanceProperty`** (update_event_test.go:232) -- FIXED. Captures PATCH request body and asserts it does NOT contain `singleValueExtendedProperties`.
