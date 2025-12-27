#!/bin/sh
set -eu

backup_dir="${BACKUP_DIR:-/backups}"
interval="${BACKUP_INTERVAL_SECONDS:-3600}"
retention_days="${BACKUP_RETENTION_DAYS:-4}"

mkdir -p "$backup_dir"
umask 077

while true; do
  timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
  backup_file="${backup_dir}/${PGDATABASE:-db}_${timestamp}.sql.gz"

  echo "Starting backup to ${backup_file}"
  if ! command -v gzip >/dev/null 2>&1; then
    echo "gzip not found; install it in the backup image"
    exit 1
  fi
  pg_dump --no-owner --no-privileges --format=plain | gzip -c > "$backup_file"
  echo "Backup finished"

  find "$backup_dir" -type f -name "*.sql.gz" -mtime "+${retention_days}" -delete

  sleep "$interval"
done
