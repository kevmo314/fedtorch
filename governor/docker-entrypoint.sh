#!/bin/sh

set -e

. $POETRY_VENV/bin/activate

exec gunicorn --bind 0.0.0.0:5000 --forwarded-allow-ips='*' wsgi:app