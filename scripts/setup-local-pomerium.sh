#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
LOCAL_DIR="$ROOT_DIR/deploy/local-pomerium"
GENERATED_DIR="$LOCAL_DIR/.generated"
ENV_FILE="$LOCAL_DIR/local.env"
CERT_DIR="$GENERATED_DIR/certs"
KEY_DIR="$GENERATED_DIR/keys"

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf 'missing required command: %s\n' "$1" >&2
        exit 1
    fi
}

random_base64() {
    openssl rand -base64 32 | tr -d '\n'
}

read_env_value() {
    if [ ! -f "$ENV_FILE" ]; then
        return 0
    fi

    awk -v key="$1" 'BEGIN { FS = "=" } $1 == key { sub(/^[^=]*=/, ""); print; exit }' "$ENV_FILE"
}

require openssl
require awk

mkdir -p "$CERT_DIR" "$KEY_DIR"
umask 077

DEX_CLIENT_ID=$(read_env_value DEX_CLIENT_ID)
DEX_TEST_PASSWORD=$(read_env_value DEX_TEST_PASSWORD)
DEX_CLIENT_SECRET=$(read_env_value DEX_CLIENT_SECRET)
VAULT_SERVICE_AUDIT_HMAC_KEY=$(read_env_value VAULT_SERVICE_AUDIT_HMAC_KEY)
POMERIUM_COOKIE_SECRET=$(read_env_value POMERIUM_COOKIE_SECRET)
POMERIUM_SHARED_SECRET=$(read_env_value POMERIUM_SHARED_SECRET)

: "${DEX_CLIENT_ID:=pomerium}"
: "${DEX_TEST_PASSWORD:=password}"
: "${DEX_CLIENT_SECRET:=$(random_base64)}"
: "${VAULT_SERVICE_AUDIT_HMAC_KEY:=$(random_base64)}"
: "${POMERIUM_COOKIE_SECRET:=$(random_base64)}"
: "${POMERIUM_SHARED_SECRET:=$(random_base64)}"

cat > "$ENV_FILE" <<EOF_ENV
DEX_CLIENT_ID=$DEX_CLIENT_ID
DEX_CLIENT_SECRET=$DEX_CLIENT_SECRET
DEX_TEST_PASSWORD=$DEX_TEST_PASSWORD
VAULT_SERVICE_AUDIT_HMAC_KEY=$VAULT_SERVICE_AUDIT_HMAC_KEY
POMERIUM_COOKIE_SECRET=$POMERIUM_COOKIE_SECRET
POMERIUM_SHARED_SECRET=$POMERIUM_SHARED_SECRET
EOF_ENV
chmod 600 "$ENV_FILE"

awk \
    -v dex_client_id="$DEX_CLIENT_ID" \
    -v dex_client_secret="$DEX_CLIENT_SECRET" \
    '{ gsub(/__DEX_CLIENT_ID__/, dex_client_id); gsub(/__DEX_CLIENT_SECRET__/, dex_client_secret); print }' \
    "$LOCAL_DIR/dex-config.template.yaml" > "$GENERATED_DIR/dex-config.yaml"

awk \
    -v dex_client_id="$DEX_CLIENT_ID" \
    -v dex_client_secret="$DEX_CLIENT_SECRET" \
    -v cookie_secret="$POMERIUM_COOKIE_SECRET" \
    -v shared_secret="$POMERIUM_SHARED_SECRET" \
    '{ gsub(/__DEX_CLIENT_ID__/, dex_client_id); gsub(/__DEX_CLIENT_SECRET__/, dex_client_secret); gsub(/__POMERIUM_COOKIE_SECRET__/, cookie_secret); gsub(/__POMERIUM_SHARED_SECRET__/, shared_secret); print }' \
    "$LOCAL_DIR/pomerium-config.template.yaml" > "$GENERATED_DIR/pomerium-config.yaml"

if [ ! -f "$CERT_DIR/pomerium-local.key" ] || [ ! -f "$CERT_DIR/pomerium-local.crt" ]; then
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout "$CERT_DIR/pomerium-local.key" \
        -out "$CERT_DIR/pomerium-local.crt" \
        -days 3650 \
        -subj "/CN=localhost.pomerium.io" \
        -addext "subjectAltName=DNS:localhost.pomerium.io,DNS:*.localhost.pomerium.io" \
        -addext "basicConstraints=critical,CA:TRUE" \
        -addext "keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign" \
        -addext "extendedKeyUsage=serverAuth" >/dev/null 2>&1
    chmod 600 "$CERT_DIR/pomerium-local.key"
fi

if [ ! -f "$KEY_DIR/pomerium-signing-key.pem" ]; then
    openssl ecparam -name prime256v1 -genkey -noout -out "$KEY_DIR/pomerium-signing-key.pem"
    chmod 600 "$KEY_DIR/pomerium-signing-key.pem"
fi

printf 'Local Pomerium files generated in %s\n' "$GENERATED_DIR"
printf 'Local environment written to %s\n' "$ENV_FILE"
printf 'Start the stack with: docker compose up --build\n'
printf 'Run the smoke test with: make smoke-pomerium\n'
