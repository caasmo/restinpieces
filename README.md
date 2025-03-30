# REST in pieces

 A collection of golang snippets (**pieces**) to build a performant and simple
 API **REST** server, trying to avoid 3-party modules as much as possible.

## Build 

the follwoing will put in public/dist html and js files, in gzip and normal versions

    go generate

for building for prodcution with static assets proper cache headers and security headers:

    go build -ldflags="-s -w" -trimpath  ./cmd/restinpieces/...

for development, without headers:

    go build -ldflags="-s -w" -trimpath -tags dev ./cmd/restinpieces/...


## TODO

[Todos](doc/TODO.md).

