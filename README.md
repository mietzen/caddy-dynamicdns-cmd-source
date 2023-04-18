Command IP source module for [caddy-dynamicdns](https://github.com/mholt/caddy-dynamicdns)
=========================

This is a IP source for [caddy-dynamicdns](https://github.com/mholt/caddy-dynamicdns). You can use it to get your IP via any command line script or program.

E.g. get your IP from your Fritz!BOX Router via this command:

```Shell
curl -s -H 'Content-Type: text/xml; charset="utf-8"' \
  -H 'SOAPAction: urn:schemas-upnp-org:service:WANIPConnection:1#GetExternalIPAddress' \
  -d '<?xml version="1.0" encoding="utf-8"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"> <s:Body> <u:GetExternalIPAddress xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1" /></s:Body></s:Envelope>' \
  "http://$FRITZ_BOX_HOSTNAME:49000/igdupnp/control/WANIPConn1" | \
  grep -Eo '\<[[:digit:]]{1,3}(\.[[:digit:]]{1,3}){3}\>'
```
[Source](https://github.com/ddclient/ddclient/blob/4458cceb1b29b4b85fbe4f38f3381a6621048d00/sample-get-ip-from-fritzbox)

## Install

You "install" this module by building your own `caddy` binary via `xcaddy`.

Example for cloudflare DNS:

```Shell
xcaddy build --with github.com/mholt/caddy-dynamicdns --with github.com/mietzen/caddy-dynamicdns-cmd-source --with github.com/caddy-dns/cloudflare 
```

## Config

Here's an example on how to run a custom command to get the IP addresses. If the command returns ipv4 and ipv6 addresses, make sure that they are comma separated.

Caddyfile config ([global options](https://caddyserver.com/docs/caddyfile/options)):

```
{
	dynamic_dns {
		provider cloudflare {env.CLOUDFLARE_API_TOKEN}
		domains {
			example.net subdomain
		}
        ip_source command echo 1.2.3.4,2004:1234::1234
        check_interval 5m
        ttl 1h
	}
}
```

Equivalent JSON config:

```jsonc
{
	"apps": {
		"dynamic_dns": {
			"dns_provider": {
				"name": "cloudflare",
				"api_token": "{env.CLOUDFLARE_API_TOKEN}"
			},
			"domains": {
				"example.net": ["subdomain"]
			},
			"ip_sources": [
				{
					"source": "command",
					"command": "echo",
					"args": ["1.2.3.4,2004:1234::1234"]
				}
			],
			"check_interval": "5m",
			"versions": {
				"ipv4": true,
				"ipv6": true
			},
			"ttl": "1h",
			"dynamic_domains": false
		}
	}
}
```
