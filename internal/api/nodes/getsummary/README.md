# getsummary

HTTP handler that returns aggregated information about all nodes for the
`/api/nodes/summary` and `/api/dedicated_servers/summary` (alias) endpoints.

## Response shape

`summaryResponse` (see `response.go`) contains:

- `total`, `enabled`, `disabled` — counters derived from the node repository.
- `online`, `offline` — counters derived from runtime gRPC status checks.
- `onlineNodes`, `offlineNodes` — per-node entries with `version` and
  `buildDate` populated when the daemon answered.

The handler is registered as `AdminOnly` in `internal/api/router.go`. Both URL
paths use the **same handler instance** so they share the cache.

## Why caching is needed

`calculateSummary` issues a gRPC `Version` request to every node in parallel,
each bounded by `connectTimeout = 500ms`. With many nodes the slowest node
dominates request latency, and the UI polls this endpoint regularly — every
operator viewing the dashboard would otherwise spawn a fresh fan-out to all
daemons.

## Caching strategy

The handler implements a **proactive-refresh** cache on top of
`internal/cache.Cache`, so cache backend follows `CACHE_DRIVER`
(in-memory by default; Redis when configured).

### Constants (handler.go)

| Name                       | Default | Meaning                                                  |
| -------------------------- | ------- | -------------------------------------------------------- |
| `defaultCacheTTL`          | `30s`   | TTL written to `cache.Set` (Redis-side or in-memory).    |
| `backgroundRefreshTimeout` | `10s`   | Hard timeout for one scheduled refresh attempt.          |
| `cacheKey`                 | `nodes:summary` | Single shared key — endpoint takes no parameters. |

`refreshDelay` returns `(cacheTTL - backgroundRefreshTimeout) * 9 / 10` —
the goroutine fires slightly earlier than `cacheTTL - backgroundRefreshTimeout`
so a worst-case-slow refresh still finishes before the cache entry expires.

### Request flow (`ServeHTTP`)

1. Authenticate the session (cache is **strictly behind the auth boundary**).
2. `getOrCompute(ctx)`:
   - `tryGet` → `cache.GetTyped[summaryResponse](cacheKey)`.
     - Hit → return immediately.
     - Miss → `computeAndCache`.
3. `computeAndCache` runs through `singleflight.Group` keyed by `cacheKey`,
   so concurrent cold-start callers share a single fan-out:
   - `nodeRepo.FindAll`
   - `calculateSummary` (parallel `Version` calls per node)
   - `cache.Set` with `WithExpiration(cacheTTL)` (best-effort; failure is
     logged, the response is still returned).
4. Write the response to the client.
5. `scheduleRefresh()` — idempotent; ensures one pending refresh exists.

### Scheduled refresh (`scheduleRefresh` + `runScheduledRefresh`)

`scheduleRefresh` is a no-op if `refreshScheduled == true`. Otherwise it
flips the flag and arms `time.AfterFunc(refreshDelay, runScheduledRefresh)`.

`runScheduledRefresh`:

1. Detaches from the request context using `context.Background()` with a
   `backgroundRefreshTimeout` cap, so a client disconnect cannot abort the
   refresh.
2. Calls `computeAndCache` (same singleflight key as user requests).
3. Resets `refreshScheduled` in `defer`, so the next request can schedule
   the next iteration.

The refresh chain **does not perpetuate by itself** — `runScheduledRefresh`
does not call `scheduleRefresh` again. It is the *next user request* that
arms the next timer. This keeps proactive work proportional to traffic: a
polling UI keeps the cache continuously warm; a long idle period lets the
chain drain naturally and the cache TTL reclaim the entry.

### State transitions

```
                   ┌──────────────────────────┐
                   │ no cached entry          │
                   └────────────┬─────────────┘
                                │ first request
                                ▼
                   ┌──────────────────────────┐
                   │ singleflight compute     │
                   │ cache.Set TTL=30s        │
                   │ scheduleRefresh()        │
                   └────────────┬─────────────┘
                                │
                                │ requests within ~27s — cache hit
                                ▼
                   ┌──────────────────────────┐
                   │ time.AfterFunc fires     │
                   │ at refreshDelay (~18s)   │
                   │ runScheduledRefresh →    │
                   │ singleflight compute →   │
                   │ cache.Set TTL=30s        │
                   └────────────┬─────────────┘
                                │
                                │ next user request → new timer armed
                                ▼
                          (loop while traffic)
```

## Behavioural guarantees

- **No stale window**: a request after `refreshDelay` finds an already
  refreshed cache entry (assuming the previous scheduled refresh succeeded).
- **No thundering herd on cold start**: 1..N concurrent first requests are
  collapsed by `singleflight` into a single fan-out.
- **No double refresh**: `refreshScheduled` flag guarantees at most one
  pending `time.AfterFunc` per handler instance at any moment.
- **Auth boundary preserved**: `ServeHTTP` returns 401 *before* touching the
  cache for unauthenticated callers. The cache contains data computed under
  authenticated control only.
- **Refresh failure is non-fatal**: a failed `computeAndCache` (e.g. repo
  error) leaves the previous cache entry intact; the entry expires by TTL
  unless a subsequent request triggers a new compute.

## Trade-offs and limitations

- **Multi-instance with in-memory cache.** With `CACHE_DRIVER=memory`, each
  API instance maintains its own copy. With `CACHE_DRIVER=redis`, all
  instances share the same key — but each instance still runs its own
  scheduled refresh, so multiple instances may briefly recompute in parallel
  on every cycle (last-write-wins via `cache.Set`).
- **No graceful refresh cancellation on shutdown.** `time.AfterFunc` returns
  a `*Timer` that we currently do not retain; pending refreshes fire even
  during process shutdown and run on `context.Background()`. The
  `backgroundRefreshTimeout` bounds how long they hold dependencies.
- **TTL is not invalidated on node create/delete.** A node added via
  `POST /api/nodes` may take up to `cacheTTL` to appear in the summary.
  Acceptable for an admin dashboard; revisit if precise reflection becomes
  important.
- **Cache.Set type round-trip.** Redis serialises through JSON, so
  `cache.GetTyped[summaryResponse]` re-marshals on read. Negligible for this
  small payload.

## Testing

`handler_test.go` covers:

- Auth check, response shapes, calculator correctness — original test cases.
- `TestHandler_CachesFreshResponse` — sequential requests within the refresh
  window result in a single compute.
- `TestHandler_ProactivelyRefreshesBeforeExpiry` — with shrunk
  `cacheTTL` / `backgroundRefreshTimeout`, the scheduled refresh fires
  before TTL expires (verified via `assert.Eventually` on
  `mockStatusService.callCount`).
- `TestHandler_ConcurrentColdStartCollapses` — 10 parallel cold-start
  callers result in exactly one compute round (singleflight).
- `TestHandler_ScheduledRefreshErrorPreservesCache` — when the underlying
  repository starts returning errors, `runScheduledRefresh` does not
  overwrite the previously cached value.
- `TestHandler_NotAuthenticatedDoesNotConsultCache` — 401 short-circuits
  before any cache or status call.

Run:

```bash
go test -race ./internal/api/nodes/getsummary/...
golangci-lint run ./internal/api/nodes/getsummary/...
```
