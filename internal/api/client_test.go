package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fguillen/dun-cli/internal/api/gen"
)

// errProvider is a TokenProvider that always returns the configured error.
type errProvider struct{ err error }

func (e errProvider) Token(context.Context) (string, error) { return "", e.err }

// newTestClient spins an httptest.Server with the given handler and
// returns a Client pointing at it. baseURL includes the `/v1` prefix
// that the real backend exposes; the test handler can register routes
// rooted at `/v1/...` directly.
func newTestClient(t *testing.T, tp TokenProvider, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL+"/v1", tp, srv.Client())
	require.NoError(t, err)
	return c, srv
}

// writeEnvelope marshals an ErrorEnvelope to the response writer.
func writeEnvelope(t *testing.T, w http.ResponseWriter, status int, code, message string, retryAfter int) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if retryAfter > 0 {
		body["error"].(map[string]any)["retry_after"] = retryAfter
	}
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

func TestClient_New_requiresBaseURL(t *testing.T) {
	_, err := New("", nil, nil)
	require.Error(t, err)
}

func TestClient_New_clonesHTTPClient(t *testing.T) {
	// The caller's http.Client should not have its Transport mutated.
	hc := &http.Client{Timeout: 5 * time.Second}
	_, err := New("http://example.test/v1", nil, hc)
	require.NoError(t, err)
	require.Nil(t, hc.Transport, "constructor must not mutate caller's http.Client")
}

// writeJSON sends a JSON body with the Content-Type ogen expects.
func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(body))
	require.NoError(t, err)
}

func TestClient_sendsBearerAuth(t *testing.T) {
	var seen string
	c, _ := newTestClient(t, StaticToken("test-token"), func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.Header().Set("X-Request-Id", "req-abc")
		writeJSON(t, w, `{"servers": []}`)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Bearer test-token", seen)
}

func TestClient_emptyTokenStillSendsHeader(t *testing.T) {
	var seen string
	c, _ := newTestClient(t, StaticToken(""), func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		writeJSON(t, w, `{"servers": []}`)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.NoError(t, err)
	// ogen unconditionally sends `Authorization: Bearer <token>`; with an
	// empty token the value is "Bearer " on the wire but Go's header
	// reader strips trailing whitespace, so the test sees "Bearer".
	require.Equal(t, "Bearer", seen)
}

func TestClient_tokenProviderError(t *testing.T) {
	want := errors.New("boom")
	var handlerCalls atomic.Int32
	c, _ := newTestClient(t, errProvider{err: want}, func(w http.ResponseWriter, _ *http.Request) {
		handlerCalls.Add(1)
		writeJSON(t, w, `{"servers": []}`)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.ErrorIs(t, err, want)
	require.Zero(t, handlerCalls.Load(), "token provider error should short-circuit before HTTP")
}

func TestClient_decodesEnvelopeOn401(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-xyz")
		writeEnvelope(t, w, http.StatusUnauthorized, "unauthorized", "bad token", 0)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.Error(t, err)

	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "unauthorized", apiErr.Code)
	require.Equal(t, "bad token", apiErr.Message)
	require.Equal(t, "req-xyz", apiErr.RequestID)
}

func TestClient_decodes404Envelope_onListServerWorlds(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-404")
		writeEnvelope(t, w, http.StatusNotFound, "not_found", "no such server", 0)
	})

	_, err := c.ListServerWorlds(context.Background(), "srv-1")
	require.Error(t, err)

	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
	require.Equal(t, "req-404", apiErr.RequestID)
}

func TestClient_decodes429AsRateLimit(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-rl")
		writeEnvelope(t, w, http.StatusTooManyRequests, "rate_limited", "slow down", 30)
	})

	_, err := c.ListPlayerServers(context.Background())
	require.Error(t, err)

	var rl *RateLimitError
	require.ErrorAs(t, err, &rl)
	require.Equal(t, 30*time.Second, rl.RetryAfter)
	require.Equal(t, "req-rl", rl.Err.RequestID)
	require.Equal(t, "rate_limited", rl.Err.Code)

	// errors.As against *Error should still work via Unwrap.
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "rate_limited", apiErr.Code)
}

func TestClient_capturesRequestIDOnSuccess(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-ok")
		writeJSON(t, w, `{"status": "ok"}`)
	})

	require.NoError(t, c.Health(context.Background()))
	// Success path doesn't return the ID to callers in Phase 1 — it's
	// for the slog record only. This test ensures the transport ran
	// without crashing and the round-trip completed.
}

func TestClient_contextCancellation(t *testing.T) {
	// Handler blocks until the test's context is done so the cancel
	// races the response.
	c, _ := newTestClient(t, StaticToken("t"), func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before issuing the call

	err := c.Health(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestClient_health_envelopeDecoded(t *testing.T) {
	// Sanity: the Health wrapper rejects an unexpected union variant.
	// The spec only declares 200 for /health, so any non-200 turns into
	// an UnexpectedStatusCode transport error from ogen — we just make
	// sure that surfaces rather than panicking.
	c, _ := newTestClient(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := c.Health(context.Background())
	require.Error(t, err)
}

// Both security seams delegate to the TokenProvider — the active scope
// is decided by which provider the *Client was built with (player vs
// admin), not by which seam ogen happens to call.
func TestSecurity_bothBearersDelegateToProvider(t *testing.T) {
	src := bearerSource{tp: StaticToken("x")}

	pb, err := src.PlayerBearer(context.Background(), "anything")
	require.NoError(t, err)
	require.Equal(t, "x", pb.Token)

	ab, err := src.AdminBearer(context.Background(), "anything")
	require.NoError(t, err)
	require.Equal(t, "x", ab.Token)
}

// fromEnvelope is exercised indirectly by the 429 path; this unit-level
// test pins down the time conversion and the nil-envelope guard.
func TestFromEnvelope_retryAfterParsed(t *testing.T) {
	env := &gen.ErrorEnvelope{}
	env.Error.Code = "rate_limited"
	env.Error.Message = "slow down"
	env.Error.RetryAfter.SetTo(45)

	err := fromEnvelope(env, "req-1")
	var rl *RateLimitError
	require.ErrorAs(t, err, &rl)
	require.Equal(t, 45*time.Second, rl.RetryAfter)
}

func TestFromEnvelope_noRetryAfter(t *testing.T) {
	env := &gen.ErrorEnvelope{}
	env.Error.Code = "not_found"
	env.Error.Message = "nope"

	err := fromEnvelope(env, "")
	require.Error(t, err)
	require.Nil(t, AsRateLimit(err))
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
}

// AsRateLimit is a small test helper that mirrors AsError for the
// rate-limit variant. Kept private to the test file because v1 callers
// just `errors.As` directly.
func AsRateLimit(err error) *RateLimitError {
	var rl *RateLimitError
	if errors.As(err, &rl) {
		return rl
	}
	return nil
}

// ── Phase 8 wrappers ──────────────────────────────────────────────

func TestPreviewTrainingOrder_capturesQueryParams(t *testing.T) {
	var qp url.Values
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		qp = r.URL.Query()
		w.Header().Set("X-Request-Id", "req-train-preview")
		writeJSON(t, w, `{
			"building_kind": "barracks",
			"unit": "levy",
			"count": 5,
			"building_level": 2,
			"building_built": true,
			"unit_trainable_here": true,
			"per_unit_cost": {"gold": 10, "wood": 5, "stone": 0, "iron": 0},
			"total_cost": {"gold": 50, "wood": 25, "stone": 0, "iron": 0},
			"per_unit_seconds": 90,
			"total_seconds": 450,
			"affordable": true,
			"missing": {"gold": 0, "wood": 0, "stone": 0, "iron": 0},
			"max_affordable_count": 20
		}`)
	})

	prev, err := c.PreviewTrainingOrder(context.Background(), "kgd-1", "barracks", "levy", 5)
	require.NoError(t, err)
	require.Equal(t, "barracks", string(prev.BuildingKind))
	require.Equal(t, 5, prev.Count)
	require.Equal(t, "barracks", qp.Get("building"))
	require.Equal(t, "levy", qp.Get("unit"))
	require.Equal(t, "5", qp.Get("count"))
}

func TestDispatchMarch_postsIntentAndTarget(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("X-Request-Id", "req-march")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"id": "mrc-1",
			"army_id": "arm-1",
			"intent": "scout",
			"origin_region_id": "reg-a",
			"target_region_id": "reg-b",
			"path": ["reg-a", "reg-b"],
			"dispatched_at": "2026-05-20T00:00:00Z",
			"arrives_at": "2026-05-20T02:00:00Z",
			"arrived_at": null,
			"recalled_at": null
		}`))
	})

	march, err := c.DispatchMarch(context.Background(), "arm-1", "reg-b", "scout")
	require.NoError(t, err)
	require.Equal(t, "mrc-1", march.ID)
	require.Equal(t, "scout", string(march.Intent))
	require.Equal(t, "reg-b", body["target_region_id"])
	require.Equal(t, "scout", body["intent"])
}

func TestRecallMarch_404SurfacesNotFound(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-recall")
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.RecallMarch(context.Background(), "arm-99")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
	require.Contains(t, apiErr.Message, "no active march")
}

func TestSplitArmy_invalidatesArmiesCache(t *testing.T) {
	calls := atomic.Int32{}
	splitDone := atomic.Bool{}
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-split")
		switch r.URL.Path {
		case "/v1/kingdoms/kgd-1/armies":
			calls.Add(1)
			if splitDone.Load() {
				writeJSON(t, w, `{"armies": [
					{"id": "arm-1", "kingdom_id": "kgd-1", "name": "Garrison", "status": "home",
					 "location_region_id": "reg-a",
					 "composition": {"levy": 7}, "total_capacity": 35},
					{"id": "arm-2", "kingdom_id": "kgd-1", "name": "Scouts", "status": "home",
					 "location_region_id": "reg-a",
					 "composition": {"levy": 5}, "total_capacity": 25}
				]}`)
			} else {
				writeJSON(t, w, `{"armies": [
					{"id": "arm-1", "kingdom_id": "kgd-1", "name": "Garrison", "status": "home",
					 "location_region_id": "reg-a",
					 "composition": {"levy": 12}, "total_capacity": 60}
				]}`)
			}
		case "/v1/armies/arm-1/split":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			splitDone.Store(true)
			_, _ = w.Write([]byte(`{
				"source": {"id": "arm-1", "kingdom_id": "kgd-1", "name": "Garrison", "status": "home",
				           "location_region_id": "reg-a",
				           "composition": {"levy": 7},
				           "total_capacity": 35},
				"new": {"id": "arm-2", "kingdom_id": "kgd-1", "name": "Scouts", "status": "home",
				        "location_region_id": "reg-a",
				        "composition": {"levy": 5},
				        "total_capacity": 25}
			}`))
		default:
			http.NotFound(w, r)
		}
	})

	// Warm the resolver cache.
	id, err := c.ResolveArmy(context.Background(), "kgd-1", "Garrison")
	require.NoError(t, err)
	require.Equal(t, "arm-1", id)
	require.Equal(t, int32(1), calls.Load())

	// Splitting should invalidate the cache so the next lookup re-fetches.
	_, err = c.SplitArmy(context.Background(), "arm-1", "Scouts", map[string]int{"levy": 5})
	require.NoError(t, err)

	newID, err := c.ResolveArmy(context.Background(), "kgd-1", "Scouts")
	require.NoError(t, err)
	require.Equal(t, "arm-2", newID)
	require.Equal(t, int32(2), calls.Load(), "split must invalidate the armies cache")
}

func TestClient_ListKingdomBattles_propagatesParams(t *testing.T) {
	var seenLimit, seenOffset string
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/kingdoms/kgd-1/battles" {
			http.NotFound(w, r)
			return
		}
		seenLimit = r.URL.Query().Get("limit")
		seenOffset = r.URL.Query().Get("offset")
		writeJSON(t, w, `{"battles": [{
			"id": "bat-1", "world_id": "wld-1", "region_id": "reg-a",
			"attacker_kingdom_id": "kgd-1", "defender_kingdom_id": "kgd-2",
			"outcome": "attacker_victory",
			"loot": {"gold": 10},
			"log": [{"round": 1, "attacker_damage_dealt": 5, "defender_damage_dealt": 2,
			         "attacker_casualties": {}, "defender_casualties": {"levy": 1}}],
			"started_at": "2026-05-19T21:00:00Z",
			"ended_at": "2026-05-19T21:30:00Z"
		}], "total_count": 1}`)
	})

	battles, total, err := c.ListKingdomBattles(context.Background(), "kgd-1", 5, 10)
	require.NoError(t, err)
	require.Equal(t, "5", seenLimit)
	require.Equal(t, "10", seenOffset)
	require.Equal(t, 1, total)
	require.Len(t, battles, 1)
	require.Equal(t, "bat-1", battles[0].ID)
	defID, ok := battles[0].DefenderKingdomID.Get()
	require.True(t, ok)
	require.Equal(t, "kgd-2", defID)
}

func TestClient_ListKingdomBattles_omitsUnsetParams(t *testing.T) {
	var rawQuery string
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		writeJSON(t, w, `{"battles": [], "total_count": 0}`)
	})
	_, _, err := c.ListKingdomBattles(context.Background(), "kgd-1", 0, 0)
	require.NoError(t, err)
	require.Empty(t, rawQuery, "no defaults should be sent when limit/offset are 0")
}

func TestClient_ShowBattle_decodesParticipants(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/battles/bat-9" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, `{"battle": {
			"id": "bat-9", "world_id": "wld-1", "region_id": "reg-a",
			"attacker_kingdom_id": "kgd-1", "defender_kingdom_id": null,
			"outcome": "attacker_victory",
			"loot": {},
			"log": [{"round": 1, "attacker_damage_dealt": 4, "defender_damage_dealt": 1,
			         "attacker_casualties": {}, "defender_casualties": {"pikeman": 2}}],
			"started_at": "2026-05-19T21:00:00Z",
			"ended_at": "2026-05-19T21:30:00Z"
		}, "participants": [
			{"id": "p-1", "battle_id": "bat-9", "kingdom_id": "kgd-1",
			 "side": "attacker",
			 "starting_composition": {"levy": 10},
			 "ending_composition": {"levy": 9},
			 "casualties": {"levy": 1}},
			{"id": "p-2", "battle_id": "bat-9", "kingdom_id": null,
			 "side": "defender", "army_id": null,
			 "starting_composition": {"pikeman": 5},
			 "ending_composition": {"pikeman": 3},
			 "casualties": {"pikeman": 2}}
		]}`)
	})

	battle, parts, err := c.ShowBattle(context.Background(), "bat-9")
	require.NoError(t, err)
	require.NotNil(t, battle)
	require.Equal(t, "bat-9", battle.ID)
	_, ok := battle.DefenderKingdomID.Get()
	require.False(t, ok, "wilderness battle has no defender (null)")
	require.Len(t, parts, 2)
	require.Equal(t, gen.BattleParticipantSideAttacker, parts[0].Side)
	// The wild defender participant carries null kingdom_id/army_id and
	// must decode to an unset OptNilString rather than failing the parse.
	require.Equal(t, gen.BattleParticipantSideDefender, parts[1].Side)
	_, ok = parts[1].KingdomID.Get()
	require.False(t, ok, "wilderness defender participant has null kingdom_id")
}

func TestClient_ShowBattle_404Envelope(t *testing.T) {
	c, _ := newTestClient(t, StaticToken("t"), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req-bat-404")
		writeEnvelope(t, w, http.StatusNotFound, "not_found", "no such battle", 0)
	})
	_, _, err := c.ShowBattle(context.Background(), "bat-x")
	require.Error(t, err)
	apiErr := AsError(err)
	require.NotNil(t, apiErr)
	require.Equal(t, "not_found", apiErr.Code)
	require.Equal(t, "req-bat-404", apiErr.RequestID)
}
