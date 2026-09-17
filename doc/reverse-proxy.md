# Running Behind a Reverse Proxy

A reverse proxy is a server that stands in front of your application and forwards requests to it. Visitors connect to the proxy, and the proxy connects to your application. Because the application talks to the proxy rather than to the visitor, the address of the connection belongs to the proxy, and details that describe the visitor, such as the visitor's real address or whether the visitor used HTTPS, travel in request headers instead.

Two kinds of proxy are common. A local proxy runs on your own machine or network, for example nginx or Caddy, and usually forwards plain HTTP to the application. A global proxy sits between every visitor and your server, for example Cloudflare, and can also handle TLS for you.

This document describes the settings to review for both setups, the values to use with nginx and with Cloudflare, and how to check the result. Every `ripc` command below runs in the application shell described in [Post-Deploy Configuration](post-deploy-config.md). After changing a setting, apply it with `sudo systemctl reload <app-name>`; the settings here are read on each request, so no restart is needed.

The document grows as the framework gains proxy features. It currently covers the visitor's address.

## Let the application see the visitor's address

A request that arrives through a proxy carries the proxy's address in the connection. Without a setting, the application would log and count the proxy as if it were every visitor: the request log shows one address for everyone, the IP blocker treats all traffic as one client, and the metrics allowed list matches the proxy instead of the visitor.

The setting that fixes this is `server.client_ip_proxy_header`. It is empty by default, and an empty value means "use the address of the connection". Set it to the name of the header your proxy fills with the visitor's address.

### nginx

Send the visitor's address from nginx:

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

Then set the same header name in the application:

```bash
ripc set server.client_ip_proxy_header X-Forwarded-For
```

`X-Real-IP` works as well; configure it in nginx with `proxy_set_header X-Real-IP $remote_addr;` and set the same name in the application. When a header contains several addresses separated by commas, the application uses the first one.

### Cloudflare

Set the header Cloudflare uses:

```bash
ripc set server.client_ip_proxy_header CF-Connecting-IP
```

Cloudflare writes exactly one address into `CF-Connecting-IP`, which makes it more reliable than `X-Forwarded-For`. Cloudflare adds the header only when the DNS record is proxied, the orange cloud. On a DNS-only record, the grey cloud, visitors reach your server directly and the application already sees the real address, so the setting does nothing.

### Check it worked

```bash
ripc get server.client_ip_proxy_header
ripc log tail
```

`ripc get` shows the configured header name. After a reload, `ripc log tail` shows the `remote_ip` field of new requests; it must contain the visitor's address, which differs between visitors, instead of the proxy's address, which is the same for everyone.
