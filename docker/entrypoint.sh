#!/bin/sh
set -eu

app="/home/site/wwwroot/app"

if [ "$#" -gt 0 ]; then
  exec "$app" "$@"
fi

worker_id="${WEBSITE_POD_NAME:-${CONTAINER_NAME:-}}"
if [ -z "$worker_id" ]; then
  echo "WEBSITE_POD_NAME or CONTAINER_NAME is required when worker arguments are not supplied." >&2
  exit 64
fi

functions_uri="${FUNCTIONS_WORKER_PROXY_URI:-http://127.0.0.1:50054}"
request_id="${FUNCTIONS_WORKER_REQUEST_ID:-}"
if [ -z "$request_id" ] && [ -r /proc/sys/kernel/random/uuid ]; then
  request_id="$(cat /proc/sys/kernel/random/uuid)"
fi
if [ -z "$request_id" ]; then
  echo "FUNCTIONS_WORKER_REQUEST_ID is required when a request ID cannot be generated." >&2
  exit 64
fi

max_message_length="${FUNCTIONS_GRPC_MAX_MESSAGE_LENGTH:-134217728}"
case "$max_message_length" in
  *[!0-9]*|'')
    echo "FUNCTIONS_GRPC_MAX_MESSAGE_LENGTH must be a positive integer." >&2
    exit 64
    ;;
esac

exec "$app" \
  --functions-uri "$functions_uri" \
  --functions-worker-id "$worker_id" \
  --functions-request-id "$request_id" \
  --functions-grpc-max-message-length "$max_message_length"
