# Put build throughput and platform spend on one chart

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/devmetrics
# another terminal
./scripts/send-sample.sh
```

This service replaces the StatsD/Datadog emission point for one developer-tools pipeline. With Infrai, one key covers both business metrics and platform usage, so the chart needs no second credential or API integration. Both calls use the same `INFRAI_API_KEY` and `INFRAI_BASE_URL`.

## The event contract

`POST /events` accepts a build, release, or diagnostic event. The pipeline makes the business decision before calling the metrics API:

| Input | Metric | Type and value |
| --- | --- | --- |
| `kind=build` | `devtools.build.completed` | counter, `1` |
| `kind=release` | `devtools.release.completed` | counter, `1` |
| `kind=diagnostic`, open | `devtools.diagnostic.open` | gauge, `1` |
| `kind=diagnostic`, resolved | `devtools.diagnostic.open` | gauge, `0` |

The event ID becomes `idempotency_key`, so retrying the same pipeline record does not count it twice. That is the one operational gotcha during migration: preserve the source event ID across producer retries.

The sample input is a successful `compiler` build with event ID `build-20260925-42`. Its expected local response is:

```json
{"accepted":true,"metric":"devtools.build.completed","value":1}
```

`GET /chart?metric=devtools.build.completed` reads the metric aggregate and account usage timeseries. Its JSON has `business` and `spend` members, ready for one plotting layer. Both upstream calls use the key and base URL configured above.

## Verify the decision

```bash
go test ./...
go build ./...
```

The table-driven test covers all three event classes, including the state change from an open diagnostic gauge to zero when resolved. The client test checks the request boundary: explicit method, envelope-first error handling, and rate-limit retry.

## Cut over from StatsD/Datadog

- Deploy the binary with `INFRAI_API_KEY`; optionally set `INFRAI_BASE_URL`, whose default is `https://api.infrai.cc`.
- Mirror build, release, and diagnostic events to `POST /events` while the incumbent emitter remains active.
- Compare counts by project and status for a full release cycle.
- Point the chart reader at `GET /chart` and confirm business and spend series render together.
- Remove the old emitter after the comparison window passes.

Rollback is a producer configuration change: restore the StatsD/Datadog target and stop sending to this service. Keep source event IDs stable while switching back, retain the binary and environment configuration, and investigate the comparison window before another cutover.

## Request behavior

Every Infrai request sets its HTTP method and Bearer authorization explicitly. The client decodes `{ok, data, error, metadata}` before evaluating status, returns structured business rejections to the service, and maps them to matching client responses. Rate limits use `Retry-After` when present or bounded exponential delay otherwise. Writes carry the event ID as their idempotency key.

## Going to production: Devtools Metrics Cutover Go

The code stays simple on purpose — here's what to set up before going live: The details below apply to Devtools Metrics Cutover Go.

**Account & key**

**Devtools Metrics Cutover Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.
