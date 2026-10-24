#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/events \
  --header 'Content-Type: application/json' \
  --data '{"event_id":"build-20260925-42","kind":"build","project":"compiler","status":"succeeded","occurred_at":"2026-09-25T08:00:00Z"}'

curl --fail-with-body --request GET 'http://localhost:8080/chart?metric=devtools.build.completed'
