#!/bin/sh
set -eu

config_path=/tmp/envoy.yaml

if [ "${ENVOY_RENDER_MODE:-false}" = "true" ]; then
  : "${PORT:?PORT is required in Render mode}"
  : "${AUTH_SERVICE_HOST:?AUTH_SERVICE_HOST is required in Render mode}"
  : "${AUTH_SERVICE_PORT:?AUTH_SERVICE_PORT is required in Render mode}"
  : "${CORS_ALLOWED_ORIGIN_REGEX:?CORS_ALLOWED_ORIGIN_REGEX is required in Render mode}"

  envsubst '${PORT} ${AUTH_SERVICE_HOST} ${AUTH_SERVICE_PORT} ${CORS_ALLOWED_ORIGIN_REGEX}' \
    < /etc/envoy/envoy.render.yaml.template \
    > "${config_path}"
else
  : "${CORS_ALLOWED_ORIGIN_REGEX:?CORS_ALLOWED_ORIGIN_REGEX is required}"
  envsubst '${CORS_ALLOWED_ORIGIN_REGEX}' \
    < /etc/envoy/envoy.yaml \
    > "${config_path}"
fi

exec /usr/local/bin/envoy \
  -c "${config_path}" \
  --service-cluster money-tracking-api-gateway \
  --log-level "${ENVOY_LOG_LEVEL:-info}" \
  "$@"
