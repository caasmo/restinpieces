# Running Behind a Reverse Proxy

A reverse proxy is a server that stands in front of your application and forwards requests to it. Visitors connect to the proxy, and the proxy connects to your application. Because the application talks to the proxy rather than to the visitor, the address of the connection belongs to the proxy, and details that describe the visitor, such as the visitor's real address or whether the visitor used HTTPS, travel in request headers instead.

Two kinds of proxy are common. A local proxy runs on your own machine or network, for example nginx or Caddy, and usually forwards plain HTTP to the application. A global proxy sits between every visitor and your server, for example Cloudflare, and can also handle TLS for you.

Every `ripc` command below runs in the application shell described in [Post-Deploy Configuration](post-deploy-config.md). After changing a setting, apply it with `sudo systemctl reload <app-name>`; settings that are read at startup need `sudo systemctl restart <app-name>` instead.

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

## Prove the connection comes from the proxy (mTLS)

The visitor's address is only as trustworthy as the connection it arrives on: anyone who can reach the application directly can send the same header with any address. Mutual TLS, or mTLS, closes that gap: with normal TLS only the server proves who it is, while with mTLS the other side, here the proxy, must prove who it is as well. Cloudflare calls its side of this Authenticated Origin Pulls; nginx calls it client certificate verification.

Fill `server.tls.mtls_certificates` with the certificates the application should accept, written as one text value (a PEM bundle; several certificates may be concatenated). The setting is empty by default, and an empty value means no certificate is asked for. It is read when the server starts, so restart the service after changing it. When the setting is filled and `server.tls.enabled` is `false`, the application refuses to start, because there is no TLS handshake to check the certificate in.

A client whose certificate is missing, expired, or not signed by one of the stored certificates is stopped during the TLS handshake. The request never reaches the HTTP layer, so it cannot influence the request log, the IP blocker, or the visitor's address.

### Cloudflare: Authenticated Origin Pulls

Turn on Authenticated Origin Pulls in the Cloudflare dashboard under SSL/TLS → Origin Server, and make sure Cloudflare connects to your origin over HTTPS with the encryption mode Full or Full (strict) under SSL/TLS → Overview. The origin is the server that runs the application. Then give the application Cloudflare's certificate, with `@` making `ripc set` read the value from the file:

```bash
curl -o /tmp/authenticated_origin_pull_ca.pem https://developers.cloudflare.com/ssl/static/authenticated_origin_pull_ca.pem
ripc set server.tls.mtls_certificates @/tmp/authenticated_origin_pull_ca.pem
sudo systemctl restart <app-name>
```

Cloudflare now presents a certificate signed by that bundle on every connection it opens to your origin. A connection straight to the origin's address has no such certificate and fails during the handshake, which is what makes `CF-Connecting-IP` trustworthy.

### nginx

nginx can present a certificate of its own when it forwards to the application. Create one, point nginx at it, and tell the application to accept it.

```bash
sudo openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -subj "/CN=nginx-reverse-proxy" -keyout /etc/nginx/ssl/nginx-client.key -out /etc/nginx/ssl/nginx-client.crt
```

In the nginx `location` block, forward over HTTPS and use the certificate:

```nginx
location / {
    proxy_pass https://127.0.0.1:443;
    proxy_ssl_certificate     /etc/nginx/ssl/nginx-client.crt;
    proxy_ssl_certificate_key /etc/nginx/ssl/nginx-client.key;
}
```

Replace the `proxy_pass` port with the application's TLS listen address, `server.addr`. Then store the certificate in the application:

```bash
ripc set server.tls.mtls_certificates @/etc/nginx/ssl/nginx-client.crt
sudo systemctl restart <app-name>
```

The example certificate is self-signed, so storing the certificate itself is enough. When a certificate authority signs nginx's certificate instead, store the authority's certificate; the application then accepts every certificate that authority signs.

### Check it worked

After the restart, the startup log shows `client_certificate_check=required`. From a machine that is not the proxy, connect to the origin:

```bash
openssl s_client -connect <origin-address>:443 -servername example.com
```

The handshake ends with a TLS alert about a required certificate, and no HTTP response appears. Through the proxy the site still loads. For the nginx setup, confirm the good path by adding its certificate to the same command:

```bash
openssl s_client -connect 127.0.0.1:443 -servername example.com -cert /etc/nginx/ssl/nginx-client.crt -key /etc/nginx/ssl/nginx-client.key
```

This handshake completes.
