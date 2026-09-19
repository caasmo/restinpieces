# ACME certificates

ACME issues TLS certificates and renews them automatically. The framework has no ACME code, to keep dependencies minimal: it only defines the `[acme]` settings in [config/acme.go](../config/acme.go), which the [restinpieces-acme](https://github.com/caasmo/restinpieces-acme) program reads and writes. You configure those settings with `ripc` in the main app configuration. To issue a certificate, ACME must confirm you control each domain; the only supported method is `dns-01`, which confirms control by publishing a DNS TXT record.

## Content

- [Enabling certificates](#enabling-certificates)
- [Deactivating certificates](#deactivating-certificates)
- [Configuration](#configuration)
  - [`acme.account`](#acmeaccount)
  - [Shared settings](#shared-settings)
  - [`acme.dns-01.<label>`](#acmedns-01label)
  - [`acme.certificate` and `acme.private_key`](#acmecertificate-and-acmeprivate_key)
- [Renewal on a schedule](#renewal-on-a-schedule)

## Enabling certificates

Each DNS setup gets one entry. Scaffold it with defaults, then fill in the settings:

```bash
ripc scaffold acme-dns-01 cf
ripc set acme.dns-01.cf.provider cloudflare
ripc set acme.dns-01.cf.credentials.api_token @/path/to/cloudflare-token.txt
ripc set acme.account.email 'hostmaster@example.com'
ripc set acme.account.key @/path/to/account.key
ripc set acme.domains '["example.com", "*.example.com"]'
ripc set acme.ca_directory_url 'https://acme-v02.api.letsencrypt.org/directory'
```

Scaffold creates the entry with an empty `provider` and an empty `api_token`. An empty `provider` deactivates the entry, so set both to activate it. The label (`cf` above) is a name you choose; it must not contain whitespace or `.`. Generate the account key with:

```bash
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out account.key
```

## Deactivating certificates

To deactivate one entry, empty its `provider`:

```bash
ripc set acme.dns-01.cf.provider ""
```

To deactivate the whole section, empty the account key:

```bash
ripc set acme.account.key ""
```

## Configuration

Configuration lives under `[acme]` in [config/acme.go](../config/acme.go).

| Setting | Description |
|---|---|
| `account` | Account email and private key. |
| `domains`, `ca_directory_url`, `profile`, `remaining_lifetime_fraction` | Certificate and renewal settings. |
| `dns-01` | `dns-01` entries, one per DNS setup. The only challenge TOML table. |
| `certificate`, `private_key` | Certificate chain and private key written by restinpieces-acme; you never set them by hand. |

### `acme.account`

| Setting | Type | Default | Description |
|---|---|---|---|
| `email` | string | — (required once the key is set) | Contact address for the account. |
| `key` | string | `""` (section deactivated) | Account private key in PEM form (ECDSA or RSA). Empty deactivates the whole section. |

### Shared settings

These settings sit directly under `[acme]`, not inside a challenge TOML table.

| Setting | Type | Default | Description |
|---|---|---|---|
| `domains` | list of strings | — (required once the key is set) | Names the certificate must cover, for example `["example.com", "*.example.com"]`. |
| `ca_directory_url` | string | — (required once the key is set) | ACME server directory URL. |
| `profile` | string | `""` (CA picks its default) | ACME profile name from the CA directory. |
| `remaining_lifetime_fraction` | float | `0.25` | Renew when this share of the lifetime remains. Must be greater than 0 and less than 1. |

### `acme.dns-01.<label>`

| Setting | Type | Default | Description |
|---|---|---|---|
| `provider` | string | `""` (deactivated) | DNS provider implementation, for example `cloudflare`. Empty deactivates this entry. |
| `credentials` | map of strings | `{"api_token": ""}` | Values the DNS provider needs; restinpieces-acme defines their names. Must not be empty once a provider is set. |

Once the account key is set, exactly one `dns-01` entry must set a non-empty `provider`.

### `acme.certificate` and `acme.private_key`

restinpieces-acme writes the new certificate chain into `acme.certificate` and the matching private key into `acme.private_key`; you never write them by hand. Activate them with:

```bash
ripc update tls
```

## Renewal on a schedule

Certificates renew only while this scheduler job runs. The job runs code from [restinpieces-acme](https://github.com/caasmo/restinpieces-acme); add it with:

```bash
ripc scaffold job acme_cert
ripc set scheduler.jobs.acme_cert.job_type acme_cert
ripc set scheduler.jobs.acme_cert.activated true
ripc set scheduler.jobs.acme_cert.interval 1h
```
