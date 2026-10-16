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

## argon_probe

Measures the server and how long one password hash takes, then prints a recommended `limit` and `burst` for the `password_hashing` limiter, at 10%, 30%, 50%, 70% and 90% of the machine. It reads no config, takes no arguments and changes nothing.

```sh
go build -o /tmp/argon-probe scripts/argon_probe.go
```

```sh
./ripdep cp <host> <project> /tmp/argon-probe
```

Run it on the server:

```sh
./argon-probe
```

## adduser

Creates or updates a password user directly in the application database at the conventional path data/app.db (doc/layout-best-practices.md), so a working login exists without going through registration and email verification. The email and password are the two arguments; both are normalised and validated with the app's own rules, the password is hashed with the same argon2id the app verifies with, and the account is marked verified. Running it again with the same email just replaces the password.

```sh
go build -o /tmp/adduser scripts/adduser.go
```

```sh
./ripdep cp <host> <project> /tmp/adduser
```

Run it from the project home:

```sh
./adduser you@example.com '<password>'
```
