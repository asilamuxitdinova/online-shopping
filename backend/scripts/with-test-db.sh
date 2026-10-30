#!/bin/sh
# Runs a command with TEST_DATABASE_URL pointing at a throwaway PostgreSQL
# server in a temporary directory, removed again afterwards. Needs the
# PostgreSQL server binaries (initdb, pg_ctl) but not a running server or sudo.
#
#   sh scripts/with-test-db.sh go test ./...
set -eu

PG_BIN=${PG_BIN:-$(ls -d /usr/lib/postgresql/*/bin 2>/dev/null | sort -V | tail -n 1)}
if [ ! -x "$PG_BIN/initdb" ]; then
	echo "PostgreSQL server binaries not found; set PG_BIN=/path/to/postgresql/bin" >&2
	exit 1
fi
PORT=${TEST_PG_PORT:-55432}

dir=$(mktemp -d)
cleanup() {
	"$PG_BIN/pg_ctl" -D "$dir/data" -m immediate stop >/dev/null 2>&1 || true
	rm -rf "$dir"
}
trap cleanup EXIT INT TERM

"$PG_BIN/initdb" -D "$dir/data" -U postgres --auth=trust >/dev/null
# fsync is off because the data is thrown away; no Unix socket, TCP on localhost only.
"$PG_BIN/pg_ctl" -D "$dir/data" -l "$dir/postgres.log" -w \
	-o "-p $PORT -k '' -c listen_addresses=localhost -c fsync=off" start >/dev/null

TEST_DATABASE_URL="postgres://postgres@localhost:$PORT/postgres?sslmode=disable" "$@"
