#!/bin/sh
set -eu

: "${JWT_PRIVATE_KEY_PATH:?JWT_PRIVATE_KEY_PATH is required}"
: "${JWT_PUBLIC_KEYS:?JWT_PUBLIC_KEYS is required}"

runtime_key_dir=/tmp/auth-service-keys
mkdir -p "${runtime_key_dir}"
chmod 0700 "${runtime_key_dir}"

runtime_private_key="${runtime_key_dir}/jwt-private.pem"
cp "${JWT_PRIVATE_KEY_PATH}" "${runtime_private_key}"
chown appuser:appgroup "${runtime_private_key}"
chmod 0400 "${runtime_private_key}"

remaining="${JWT_PUBLIC_KEYS}"
runtime_public_keys=""
key_index=0
while [ -n "${remaining}" ]; do
  case "${remaining}" in
    *";"*)
      entry=${remaining%%;*}
      remaining=${remaining#*;}
      ;;
    *)
      entry=${remaining}
      remaining=""
      ;;
  esac

  key_id=${entry%%=*}
  source_path=${entry#*=}
  if [ -z "${key_id}" ] || [ "${source_path}" = "${entry}" ] || [ -z "${source_path}" ]; then
    echo "Invalid JWT_PUBLIC_KEYS entry" >&2
    exit 1
  fi

  key_index=$((key_index + 1))
  runtime_public_key="${runtime_key_dir}/jwt-public-${key_index}.pem"
  cp "${source_path}" "${runtime_public_key}"
  chown appuser:appgroup "${runtime_public_key}"
  chmod 0444 "${runtime_public_key}"

  if [ -n "${runtime_public_keys}" ]; then
    runtime_public_keys="${runtime_public_keys};"
  fi
  runtime_public_keys="${runtime_public_keys}${key_id}=${runtime_public_key}"
done

chown appuser:appgroup "${runtime_key_dir}"
export JWT_PRIVATE_KEY_PATH="${runtime_private_key}"
export JWT_PUBLIC_KEYS="${runtime_public_keys}"

exec su-exec appuser:appgroup /app/auth-service "$@"
