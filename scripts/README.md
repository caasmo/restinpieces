# scripts

## smtp_probe

Sends one OTP mail through the configured mail provider, so a provider switch can be checked before users are affected. Config comes from the conventional paths data/app.db and age.key (doc/layout-best-practices.md). The recipient address is the only argument.

```sh
go build -o /tmp/smtp-probe scripts/smtp_probe.go
```

```sh
./ripdep cp <host> <project> /tmp/smtp-probe
```

Run it from the project home:

```sh
./smtp-probe you@example.com
```

## s3_probe

Uploads, reads back, heads, lists and deletes one throwaway object in the given bucket, so the storage settings can be checked end to end. Endpoint, region and keys come from the s3 settings in the config at the conventional paths data/app.db and age.key (doc/layout-best-practices.md). The bucket name is the only argument.

```sh
go build -o /tmp/s3-probe scripts/s3_probe.go
```

```sh
./ripdep cp <host> <project> /tmp/s3-probe
```

Run it from the project home:

```sh
./s3-probe <bucket>
```
