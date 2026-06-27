#!/bin/sh
set -e
cat > /usr/share/nginx/html/config.js <<EOF
window.API_BASE = '${API_URL:-}';
EOF
exec /docker-entrypoint.sh nginx -g 'daemon off;'
