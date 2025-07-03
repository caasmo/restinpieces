#!/bin/sh

# USAGE:
# 	go run -exec ./setcap.sh main.go <args...>
#
# The -exec flag tells go run to use the specified <command> to execute the
# compiled binary instead of running it directly.
#
# 	go build && ./setcap.sh ./caddy <args...>
#
# but this will leave the ./caddy binary laying around.
#
sudo setcap cap_net_bind_service=+ep "$1"
"$@"

