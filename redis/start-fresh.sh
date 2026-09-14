#!/bin/sh
# Start Redis Stack with an empty dataset.
#
# Anything left in the data directory by the previous run (the append-only
# write log and/or RDB snapshot) is moved into $REDIS_ARCHIVE_DIR/YYYYMMDD-HHMMSS
# before Redis starts, so every launch begins clean but no data is lost.
# The timestamp is the launch time in UTC.
set -eu

DATA_DIR="${REDIS_DATA_DIR:-/data}"
ARCHIVE_DIR="${REDIS_ARCHIVE_DIR:-/archive}"

# Keep dotfiles out of the glob; Redis writes none, but be explicit.
set -- "$DATA_DIR"/*
if [ -e "$1" ]; then
    stamp="$(date -u +%Y%m%d-%H%M%S)"
    dest="$ARCHIVE_DIR/$stamp"
    # Two launches in the same second would collide; never overwrite.
    n=1
    while [ -e "$dest" ]; do
        dest="$ARCHIVE_DIR/$stamp-$n"
        n=$((n + 1))
    done
    mkdir -p "$dest"
    mv "$@" "$dest"/
    echo "start-fresh: archived previous Redis data to $dest"
else
    echo "start-fresh: $DATA_DIR is empty, nothing to archive"
fi

exec /entrypoint.sh
