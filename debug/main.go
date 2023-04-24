package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	// plug in Caddy modules here
	_ "github.com/caddy-dns/cloudflare"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/mholt/caddy-dynamicdns"
	_ "github.com/mietzen/caddy-dynamicdns-cmd-source"
)

func main() {
	caddycmd.Main()
}
