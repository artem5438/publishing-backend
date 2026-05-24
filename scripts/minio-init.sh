#!/bin/sh
set -eu

MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://minio:9000}"
MINIO_ROOT_USER="${MINIO_ROOT_USER:-minioadmin}"
MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-minioadmin123}"
MINIO_BUCKET="${MINIO_BUCKET:-publishing-media}"

echo "Waiting for MinIO at ${MINIO_ENDPOINT}..."
until mc alias set local "${MINIO_ENDPOINT}" "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}" >/dev/null 2>&1; do
  sleep 2
done

mc mb "local/${MINIO_BUCKET}" --ignore-existing
mc anonymous set download "local/${MINIO_BUCKET}"

# Per-bucket CORS (mc cors set) requires MinIO AIStor. Community MinIO uses
# MINIO_API_CORS_ALLOW_ORIGIN on the minio service (see docker-compose.yml).

echo "MinIO bucket ${MINIO_BUCKET} is ready (public read)."
