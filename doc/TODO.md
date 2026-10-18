# server: unstarted daemons are stopped after a startup failure

- `Server.Run` (server/server.go): when a daemon fails to start, the start loop breaks but the shutdown loop still stops **all** registered daemons, including those never started
- their `Stop` waits on a `ShutdownDone` that never closes, burning the full `ShutdownGracefulTimeout` (default 15s) and returning a spurious "daemon failed to stop gracefully: context deadline exceeded" error
- fix: track started daemons (append after each successful `Start()`) and stop only those — already fixed in the go-daemon-runner extraction (impl-daemon-runner.md Phase 3, `startedDaemons`)
- ref: `server/server.go:184-194` (start loop), `server/server.go:262-276` (shutdown loop)


# cache: rotating cursor sweep (W/K) — reclaim expired never-read entries

- `cache/default.go` is preallocated LRU (`Get` lazy-expires, `SetWithTTL` evicts LRU tail). Expired never-read entries waste effective capacity.
- TODO: inline sweep, no goroutine, bounded `W/K`. `W = window` nodes per sweep, `K = every` K writes — e.g. `W=64, K=64` → `W/K=1` check/write, full pass every `maxEntries` writes, one constant for all levels. Hook `sweepIfDue()` at top of `SetWithTTL` (under lock); `sweep()` walks `W` slots from `cursor` (wraps), skips free, frees `expiration !=0 && fastNow()>expiration`.
- Live-vs-free still open: `used bool` vs `expiration==0` sentinel vs map lookup (Q41/Q42, you choose). `cost` not involved (Q39).
- Refs: `cache/default.go:77-79`, `192-193`, `brainstorm-remove-ristretto.md` Q10/Q22/Q33/Q40.

# server: implement MaxHeaderBytes

- add `Server.MaxHeaderBytes int` `toml:"max_header_bytes"` in `config/config.go`
- default `1<<20` in `config/default.go`, validate in `config/config_validate.go`, wire to `http.Server` in `server/server.go`
- ref: `config/config.go:Server`, `server/server.go:123`

# modernc: monitor upstream RowsColumnScanner support (Go 1.27)

- Go 1.27 adds `database/sql.RowsColumnScanner`; check if `modernc.org/sqlite` implements it
- if yes, adopt it where we scan rows (`db/databasesql/`)
- ref: `db/databasesql/users.go`, `db/databasesql/queue.go`

Plain version: does something that already exists tell you when this last ran or when it expires?
- The cert has its expiry date written on it. Read the file, compare to today, decide. Nothing to store.
- The replica file has a modification time. Same trick. That's what the replica daemon actually does.
- "Email a weekly digest Monday 9am" — nothing on disk knows a week has passed. You have to write down the last time it ran.
That's the whole distinction. One question, no jargon.
And it's not even the interesting part. Your point is the interesting part: if you do need to write something down, what you write down is when it last ran. Not the interval. The interval is config. The framework puts the interval in the row and then makes a new row every cycle — that's the mess, and it's the same mess whether the schedule lives in a daemon or a queue.

# ripc get: maybe add a --runtime flag to show what the app sees

- `ripc get` prints stored values only; `ripc dump --runtime` prints defaults merged with stored overrides (what the app actually uses)
- a `get --runtime` flag would apply the same merge then filter by path, so operators see the effective value for one key
- ref: `cmd/ripc/get.go`, `cmd/ripc/dump.go` (runtime merge precedent), `cmd/ripc/main.go` (shared `--runtime` opt), `config/default.go` (defaults source)

## ripc handlers: add invalid-flag tests

The `handleXCommand` functions still have an uncovered error branch — `printXUsage(ui.Err)` + `return err` when parsing fails with a non-`ErrHelp` error (e.g. an unknown flag). Add one invalid-flag test per command to close the remaining gap (~37% of each handler).

References:
- cmd/ripc/get.go, cmd/ripc/set.go, cmd/ripc/paths.go, cmd/ripc/dump.go, cmd/ripc/save.go, cmd/ripc/scaffold.go, cmd/ripc/migrate.go, cmd/ripc/diff.go, cmd/ripc/rollback.go
- Test files: cmd/ripc/<command>_test.go (add `TestHandle<Command>Command_InvalidFlag` next to the existing `_Help` tests)

## Document: dependency is the norm for a restinpieces-\* repo

Document in the README (or elsewhere) that a `restinpieces-*` repo is meant to be dependent on restinpieces — dependency is the norm, not a cost to engineer away.

## Remove config.Provider, consumers read the box directly

`config.Provider` is the in-between step. End state: delete `provider.go`, `core.App` owns the box, and every consumer takes `*atomic.Pointer[config.Config]` and reads `Load()` at each use.

## ripc get: misleading get strips quotes so valid TOML looks broken

`get` prints values with `%v`, so `["149.56.131.18"]` shows as `[149.56.131.18]`.

References: cmd/ripc/get.go, cmd/ripc/get_test.go, doc/ripc.md

## ripc log stats: like tail but we show stats of last mins, like total request, total error, etc

References: cmd/ripc/log_tail.go, cmd/ripc/sql.go, cmd/ripc/log_command.go, doc/ripc.md


## ripc blame: track when a particular key changes

References: config/secure.go, cmd/ripc/diff.go, cmd/ripc/update.go, cmd/ripc/get.go, cmd/ripc/paths.go, cmd/ripc/main.go, doc/ripc.md


# server: redirect server runs outside the prerouter chain, so BlockIp, BlockHost, and request logging never see port-80 traffic

- `server/server.go` — `Run` starts the redirect server with `redirectToHTTPS` as its handler, outside the prerouter chain
- `core/prerouter/block_ip.go`, `core/prerouter/block_host.go`, `core/prerouter/request_log.go` — the middleware that never sees that traffic
- `doc/post-deploy-config.md` — documents the limitation


### Maybe
- request resource rate limiting 
        - user id/ip, where to put the middleware
            - if userid, we can not put it in prerouter, as of now auth is even in each handler
            - we have a auth method, user can make a easy midleware of it in its endpoints.
            - we can even provide the middleware for the user to use 
            - leaning to separate user id and ip rate limiting
            - ip rate limiting, user id rate limiting
                - we make method isUserRateLimited to be used in handler, or in a simple middleware.
                - isIpRateLImited
            - or remove ip rate limiting enterely -> we already have a dinamic blocking, 
                - we can extend the existing blokcing algo.
                    - the sketch gives a number request per bucket -> r/s
                        - configuration has rate limitin for entire site
            - for user id, the possibilty of implement with db lookup remains, that is for pay
              for request scenarios, not protection
              the endpoint can take ip or user id. each can have different rules.
    - regular use of paid resources
    - per user request
    - batch
    - Requests per minute (RPM)
    - Requests per day (RPD)
    - middleware generates labels based on its request 
        - upon initlaization it can have labels indexes based on the rules from config
            - ex rule for presence of header H
                - labels have structure ex "H:X-my-app", default paths
            - middleware sees label of rule upon init. 
                - in request it has to build functions for the label rule, how to fill them

    - it matches the generated labels agaist each rule and 
    - it checks them in app.Cache for a block
    - if labels not blocked, it puts the matches rule ids in the channel
    - the rules ids can be a conccatenation of label+duration+auth
    - if channel full, block or ignore, based on conf
    - daemon reads from the channel
        - it deals with fixed windown, counters
        - because sequential, maps, other structure does not have lock 
        - a map of map[ruleid]map[rulewindowinsecondsbucket]map[ip/userid]counter
        - a tick remove expired bucket indexes, only the last remains.
        - if counter is max, put in app.Cache the label   
- superuser static Authorization: Bearer <token> header. Your middleware checks for this. 
    - in some routes, static, configurable not dependen on user email.
    - leverage existing jwt functions and wrap 
- SEcureConfigSote is in app just to let users of the framework use the config table with a age key and a dbpath 
    - worth it? users can create a instance itself.
        - app provides agekey and we can add the dbpath- 
        - there is nothing stateful in secureConfig, we just document use of secure store.
        - we do not even need the app to provide age and dbpath. that is normally in the entry point
    - and the server needs one for reloading, not the app. 
        - if the app has it the server has also to receive it
        - the server could provide the  object  instead
    - polluting server or app with securestore is overkill
    - agekeypath in app. Why?
        - initalization in app of secureconfig?
    - consider put agepath and dbpath in config.
        - remove crap from app
        - server has the conf provider, it can call Reload with dbPath and agePath
        - users of secureConfig: create the secure config?
            -  we still need dbconfig
        - dbConfig
- ETag or Last-Modified: Enables efficient cache validation for performance. -> no: user
    - no we are talking about html.
    - at most a weak etag like deploy tag
    - maybe max-age 1 hour in cache control
    - opinionaated, user can make its own
- block ua: cache db,  
- block jwt: cache db,  
- block referrer
- rand source in app. performacen rand

# jobs: max_attempts must be implemented — maybe in the future all jobs are defined in config and the defaults seed the framework's one-shot email sends

- `max_attempts` is stored in `job_queue` but never enforced: `StmtClaim` increments `attempts`, `MarkFailed` records the error, and nothing compares the two, so a failing job is retried forever
- `max_attempts` is policy, so it lives in config, not on the row. The row keeps only `attempts` (state). Enforcement compares the row's `attempts` against the config value at claim time; the `max_attempts` column is the leftover to delete — do not copy the value onto the row at insert time, that is config in the job table
- every job type gets one `max_attempts` value in config, keyed by job type
- the framework's internal job types are a hardcoded slice in `cmd/ripc/app_create.go`; `app create` writes one `jobs.<label>` entry per item. This is the only consumer — reload and the scheduler read the stored `jobs.<label>` entries from the DB, so `config` never imports the list and there is no import cycle
- each list item carries the label, the type constant and the default `max_attempts`
- drop the `job_type_` prefix from the constant values (e.g. `JobTypeDummy = "dummy"`); each constant stays in its own handler file. External packages (acme, backup) must follow the same convention so config stays consistent
- `ripc scaffold job` writes `max_attempts = 5` by default
- the operator can change it later, for example for `acme_cert`
- `acme_cert` is just a ready-made label the framework offers for the user to hook a job; users can ignore it and scaffold their own
- manual migration: delete all rows in `job_queue` (`DELETE FROM job_queue;`) — old prefixed `job_type` values are not rewritten
- settle: the internal types are one-shot. If they get entries in `scheduler.jobs`, `ValidateJobs` demands a positive `interval` and the scheduler will seed them recurrently — wrong. Either they get their own config section, or the scheduler must skip seeding them
- ref: `cmd/ripc/app_create.go`, `cmd/ripc/scaffold.go`, `queue/handlers/`, `config/job.go`, `config/config_validate.go`, `queue/scheduler/scheduler.go`, `db/databasesql/queue.go`, `sql/schema/app/job_queue.sql`

# scheduler: failed job keeps trying without checking conf

- `processJobs` grabs due waiting + failed rows and runs them with no `activated` check, so switching `scheduler.jobs.<label>` off does not stop rows already in line (seen live: LE 429 kept being hammered by job 7476)
- `activated` only matters for making new rows and after a success; the failure path just marks failed and the row stays grabbable, so only `ripc job rm <id>` stops it today
- direction talked: filter-then-grab — build the switched-off type list each tick and have `Claim` skip those types; one-off types are never on the list so they always run; FIFO and per-tick limit unchanged
- ref: `queue/scheduler/scheduler.go:118-132`, `db/databasesql/queue.go:26-40`

# auth: failed-login throttle for login/signup (in-house)

- per-IP and per-email failure counters in `app.Cache()`; lock after N with a TTL
- refs: `core/handler_auth_login_password.go`, `core/handler_auth_register_password.go`, `cache/default.go` (`SetWithTTL`), `core/prerouter/block_ip.go` (middleware shape, `app.ClientIP`), `queue/queue.go` (`CoolDownBucket`), `config/config.go` (`RateLimits`)

# auth: proof-of-work challenge for login/signup (in-house)

- server issues nonce+difficulty, client JS solves it, handler verifies before processing; gate signup and login after failures
- PoW is passed in HTTP headers (e.g. `X-Pow-Token`, `X-Pow-Counter`); every protected endpoint requires the header when PoW is enabled, otherwise it returns `err_pow_required`
- `VerifyPow` is called directly inside each handler, gated by config (no middleware/route injection); challenge is served by a dedicated `PowChallengeHandler`
- if disabled, handlers skip verification and the SDK ignores missing headers; on `err_pow_required` the SDK solves the challenge and retries with the built headers
- difficulty is a config value expressed in simple levels with approximate CPU seconds (like TopK settings), e.g. L1≈0.1s, L2≈0.5s, L3≈1s, L4≈2s, L5≈5s; ops can raise it under load without code changes
- SDK contains the PoW solver (browser `crypto.subtle` SHA-256, worker-based); it exposes `solvePoW()` and optional `onProgress`/duration so the UI can show a "checking your browser" page/spinner when solving exceeds ~200 ms, and otherwise stays silent
- refs: `core/handler_auth_register_password.go`, `core/handler_auth_login_password.go`, `core/prerouter/`, `restinpieces-js-sdk`

# json: update to go json v2

- migrate from `encoding/json` to `encoding/json/v2` (`go 1.25.0` in `go.mod`)
- refs: `core/auth.go`, `db/types.go`, `db/databasesql/queue.go`, `oauth2/oauth2.go`, `log/daemon.go`, `notify/discord/discord.go`, `queue/handlers/`

# ripc: log search for searching logs for pattern

- shape: `ripc log search <field> <pattern>` where field names a `data` key (e.g. `ripc log search status 307`, `ripc log search uri wp-content`)
- flag for how many rows back to look, default last 1000 (e.g. `--limit 1000`)
- no `json_extract`: SQL fetches the rows, Go unmarshals `data` and matches the pattern against the field
- searches stored rows, past logs not just the live tail
- refs: `cmd/ripc/log_command.go`, `cmd/ripc/log_tail.go`, `sql/schema/log/logs.sql` (`message`, `data` JSON, `created`)

# ripc: track all port addresses and provide maybe ripc ports

- list every listener from live config in one place: `server.addr`, `server.tls.redirect_addr`, `backup.sqlite-rsync.listen_addr`, plus `metrics.listen_addr` once it lands
- naming rule: every listener key ends in `addr`, so the command finds them by suffix instead of a hardcoded list; reuse the tree walk in `cmd/ripc/get.go` and `cmd/ripc/paths.go` (`Keys` plus `Get`, filter on last segment)
- mark off versus set, show active entries count for sqlite-rsync, and check actual listening sockets so a clash with another app on the same VPS shows as taken
- refs: `config/config.go` (`Server`), `config/backup.go` (`BackupSqliteRsync`), `cmd/ripc/get.go` (config read precedent), `cmd/ripc/paths.go` (path listing precedent)

# daemon: daemons start operation at startup, there should be a random delay

- every daemon fires its first operation the moment the server starts, so after a reboot or deploy the online backup, s3 upload, vacuum and rsync daemons all hit the database and disk at once
- stagger startup with a random initial delay (jitter) before each daemon's first run
- open: central in the server's daemon startup so all current and future daemons are covered at once, versus per-daemon before the first tick so each tunes its own
- refs: `server/server.go` (sequential daemon start), `restinpieces.go` (daemon registration)

# s3: add PutNoChunked so unknown-size bodies always send Content-Length

- `s3/put_object.go`: `PutObject` sends chunked when size is unknown; R2 rejects it with 411 MissingContentLength while AWS accepts it, so the fix is a new method, not a change to `PutObject`
- new `PutNoChunked(ctx, key, body)`: buffer unknown-size bodies to RAM up to 5 MiB, spill larger to a temp file removed after, always PUT with Content-Length
- refs: `s3/s3.go` (SigV4, UNSIGNED-PAYLOAD), `doc/s3.md` (uploader.go deliberately not ported), R2 error 10033 docs

# ripdep ripc: set s3.endpoint '' loses empty value

- `scripts/ripdep` `cmd_ripc` joins args with `$*` then interpolates unquoted into `ssh ... sh -c '... $args'`, so `set s3.endpoint ''` arrives as one arg and `ripc set` fails with missing value argument
- fix: quote each arg for the inner shell (`''` for empty), then escape once more for the outer `sh -c '...'`
- workaround: `ripc ... set s3.endpoint '""'` survives the join
- refs: `scripts/ripdep` (`cmd_ripc`), `cmd/ripc/set.go` (`parseSetArgs`)

# metrics: add a WithMetrics option

- the recorder is the only piece set after `New` (`app.SetMetrics`); give it `WithMetrics(m core.MetricsRecorder)` next to `WithNotifier` — `New` installs no default, so no nil-check
- the recorder comes from `restinpieces-metrics` in the app's main
- refs: `options.go`, `core/app.go` (`SetMetrics`, `Metrics`)

# metrics: recovered panics are not recorded

- Recovery answers a panic with a 500 after the metrics middleware has already unwound, so the line after next.ServeHTTP never runs and the response is not recorded
- decide the fix: Recovery notifies the MetricsRecorder, or the ResponseRecorder owns the recording
- ref: core/prerouter/recovery.go, core/prerouter/metrics.go, core/metrics_recorder.go

# server: cap total connections with netutil.LimitListener — deferred

- cap how many connections stay open at once with `golang.org/x/net/netutil.LimitListener`; zero or negative means no cap
- would need `max_connections` in `[server]` (default 1000) and `Run` to switch from `ListenAndServeTLS`/`ListenAndServe` to `net.Listen` plus `ServeTLS`/`Serve` so the listener can be wrapped
- refs: `server/server.go`, `config/config.go` (`Server`), `config/default.go`

# ripdep backup: the DB could be corrupt, the DB is backed up by other means, I do not see the point

- `cmd_backup` in `scripts/ripdep` is now a stub that exits with "not implemented"; the whole-home tarball code was removed
- reasons: it tarred a live `/home/<project-name>`, so the SQLite copy can be torn and is never checked, while the databases already have consistent backups in `config/backup.go` strategies, restinpieces-backup daemons, and restinpieces-litestream
- decide: drop the command for good, or rework it to take only non-reproducible files (e.g. `data/backups/` not yet pulled), skipping the live database
- refs: `scripts/ripdep` (`cmd_backup`), `config/backup.go`, `doc/multiapp.md` (local backup vs Litestream comparison)

# restinpieces-backup daemons: the idea was "users who do not want to import restinpieces can copy the structs" but implmentator and planner later forgot that by using ValidateBackup; two options, remove use of Validatebackup etc or remove the overenginereed interfaces

- the generic daemons (`New[T ...Config]`) exist so an embedder can supply their own config type; the standalone mains then import restinpieces anyway (`config.Backup`, the entry types, `config.ValidateBackup`), so the decoupling never happens
- option 1: keep the interfaces and remove the restinpieces use from the standalone paths — copy the backup structs and their validation locally, as the doc comments promise
- option 2: drop the generics and let the daemons take `config.Config` directly; the standalone mains unmarshal the TOML into it (dependency is the norm, see the entry above)
- refs: `restinpieces-backup/onlineapi/onlineapi.go`, `restinpieces-backup/vacuum/vacuum.go`, `restinpieces-backup/sqlitersync/origin/daemon.go`, `restinpieces-backup/cmd/onlineapi/daemon/main.go`, `restinpieces-backup/cmd/vacuum/daemon/main.go`, `restinpieces-backup/cmd/sqlite-rsync/origin/daemon/main.go`, `restinpieces-backup/cmd/*/restinpieces/main.go`, `config/config_validate.go`

# sqlite-rsync replica: replica can not be uses as a daemon in a framwork app; it could be useful for a float of app deploy A B C in different VMs, eacj one of A, B C have an app daemon that calls the other two as replica in orther to have distributed storage

- `ReplicaDaemon` does not satisfy `server.Daemon`: it has no `Start()`, and the interface requires `Name`/`Start`/`Stop` (`server/server.go:22`)
- `replica.New` takes a plain `replicaconfig.Config` value plus a prebuilt client (`sqlitersync/replica/daemon.go`), so a config reload cannot reach it
- the framework config has no replica section: `config.Backup` carries only the origin shape (`config/backup.go`), so the replica is configured from a separate standalone TOML
- fleet shape: each app runs the origin (serving its own databases) plus one replica daemon per peer, each with its own `origin_addr`, SSH credentials and local entry paths; a peer-keyed replica map in the framework config would make it a first-class app daemon
- client construction per peer could move inside the daemon, the way the S3 upload daemon builds its client from the config pointer on every tick (`restinpieces-backup/s3/upload/daemon.go`)
- refs: `restinpieces-backup/sqlitersync/replica/daemon.go`, `restinpieces-backup/cmd/sqlite-rsync/replica/daemon/main.go`, `restinpieces-backup/config/sqlitersync/replica`, `config/backup.go`, `server/server.go`, `restinpieces-backup/s3/upload/daemon.go`

# ripc scaffold: scaffolded block prints blank line before every field

- `defaultFieldsAndValues` (`cmd/ripc/scaffold.go`) renders via `toml.Marshal`, whose defaults sort keys alphabetically and leave `compactComments` false
- pelletier v1 writes `"\n"` before each `# comment`; every entry field carries a `comment:` tag, so every field gets a blank line above it and the block looks huge
- fix: render the block with an Encoder with `CompactComments(true)`; kills the blanks for all scaffold types, comments and values stay
- refs: `cmd/ripc/scaffold.go` (`defaultFieldsAndValues`, `scaffoldNextSteps`)

# backup.s3-upload: backup.s3-upload.app-s3.path = backup.s3-upload.app-s3.path_prefix = data/backups/app-online-app.db- just as with download i think we can remove path, path-prefix server both use cases

- `BackupS3UploadEntry` carries both `path` and `path_prefix` plus selector (`config/backup.go`); download serves both cases with one `object_key_prefix` (prefix for newest, exact key for one object)
- shape: single upload selector field with the same rule, exact path uploads one file, prefix uploads newest match, empty deactivates
- refs: `config/backup.go` (`BackupS3UploadEntry`, `BackupS3DownloadEntry`), `config/config_validate.go` (`validateBackupS3Upload`, `validateBackupS3Download`), `doc/backup.md`

# ripdep check_log_db_path_systemd: that is stupid, because for other architectures it will not work

- `check_log_db_path_systemd` (`scripts/ripdep`) runs the just-built local `bin/ripc` against the build database to read `log.batch.db_path`
- when cross-compiling the binary cannot execute locally; the failure is swallowed by the guard, so the check silently never runs for other targets
- fix: run it only when target equals host, or drop the local execution
- refs: `scripts/ripdep` (`check_log_db_path_systemd`, `cmd_build`)

# ripdep: create validate functions

- move the inline option validation in `scripts/ripdep` (`cmd_build`: the `--with-agekey` and `--with-db-local` file checks) into `validate_*` functions, the way `validate_s3_recovery_config` does for `--with-db-s3`
- the existing `check_*` helpers (`check_db_integrity`, `check_log_db_path_systemd`) should be renamed `validate_*` too
- refs: `scripts/ripdep` (`cmd_build`, the `check_*` helpers), `impl-ripdep-with-db-s3.md`

# online api: this is a mess, online api not using mtype for each prefix, it shoudl be a job no checks

- online due check uses filename timestamp (`latestBackupFiles` + `isBackupDue`), s3 upload picks latest by filesystem mtime (`fileToUpload`), so the two daemons disagree on which file is newest
- online ticks every 10m (`MaxTickInterval`) with a scan + due check; proposal is a scheduler job that just copies on fire with no checks, same shape as `s3_download`
- refs: `restinpieces-backup/internal/localcopy/daemon.go` (`MaxTickInterval`, `interval`), `restinpieces-backup/internal/localcopy/engine.go` (`latestBackupFiles`, `isBackupDue`, `handleFile`), `restinpieces-backup/internal/localcopy/latest.go` (`scanBackupDir`), `restinpieces-backup/onlineapi/onlineapi.go` (`Entries`), `restinpieces-backup/s3/upload/daemon.go` (`tickInterval`, `fileToUpload`, `uploadOne`)

# config: validate an config code is getting big, i am thinking in split each section of the config in its own file as backup.go and backup_validate.go

- `config/config_validate.go` is 877 lines: the `Validate()` dispatcher plus 27 section validators, while `config/s3.go`, `config/acme.go` and `config/job.go` already keep their validator in the section file
- first moves: `config/backup_validate.go` (all `validateBackup*`, ~265 lines) and `config/server_validate.go` (`validateServer*`, ~180 lines); keep `Validate()` and the small validators (jwt, smtp, oauth2, block\*, logger, cache, notifier, metrics) in `config_validate.go`
- tests follow the validators: `config/config_validate_test.go` (1090 lines) splits into `config/backup_validate_test.go` and `config/server_validate_test.go`
- do it as a pure-movement commit after the OAuth2 audit fixes land, so review and `git blame` stay clean
- runtime users of the section validators: `config/reload.go`, `config/config.go`

# ripc toml parser: date-time values are read as plain strings, so a set re-saves them quoted and changes their type

- `tomlValueOf` (`cmd/ripc/loadbytes_v1_polifill.go`) falls back to `string(node.Data)` for `unstable.LocalDate`, `LocalTime`, `LocalDateTime` and `DateTime`; go-toml v1's `LoadBytes` returns `toml.LocalDate`, `toml.LocalTime`, `toml.LocalDateTime` and `time.Time` for the same input, and `toml.Marshal` writes those bare
- the string fallback breaks round-trip: after a `set` (or `walk`), a date-time is re-saved as `field = "1979-05-27T07:32:00Z"` and `validateTomlAsConfig` then fails on a type mismatch
- verified: `toml.Marshal` writes all four types bare and unquoted (`d = 1979-05-27`, `instant = 1979-05-27T07:32:00Z`), and marshals the same values as strings quoted, so returning the v1 types really does close the round-trip
- verified: v2 keeps the source text in `node.Data` for every one of the four kinds, so the polyfill sees the delimiter the document actually used
- fix: return the v1 types — `toml.ParseLocalDate`, `toml.ParseLocalTime`, `toml.ParseLocalDateTime`, `time.Parse(time.RFC3339Nano, ...)` for the offset case (`unstable.DateTime` carries the offset, `LocalDateTime` does not)
- trap: TOML accepts a space as the date/time delimiter (`1979-05-27 07:32:00`) and v1's lexer loads it, but v1's own `ParseLocalDateTime` rejects it (it retries lowercase `t`, never a space) and so does `time.Parse` — normalize the delimiter before parsing, or `set` newly rejects a document v1 used to accept
- blocker: v1 1.9.5 cannot parse a bare local date followed by a newline. Its lexer (`lexer.go:355`) peeks one rune after the day digits and a `\n` is neither eof, `' '` nor `'T'`, so it errors; `d = 1979-05-27` loads only at end of input or before a comment. `toml.Marshal` writes the newline, so after this fix `set` saves a bare local date that every v1-based reader can no longer read
- the mutating commands are already safe: `set`, `add`, `remove`, `update`, `log_init`, `scaffold` and `walk` load through `NewTomlParser`, so they can read back whatever `set` wrote
- the remaining exposure is the read-only v1 readers: `get` (`cmd/ripc/get.go:60`) and `paths` (`cmd/ripc/paths.go:86`) call `toml.LoadBytes`; `dump` (`cmd/ripc/dump.go:60,78`), `diff` (`cmd/ripc/diff.go:83,86`) and `migrate` (`cmd/ripc/migrate.go:70`) call `toml.Unmarshal`. Same lexer, so all five hard-fail on a config holding a bare local date. Move them onto `NewTomlParser` in this change, or keep bare local dates as strings and accept the quote
- exposure is otherwise small: no struct in `config/` has a `time.Time` or `toml.Local*` field, so no live config can hit this. `TestTomlParser_DateTimes` does exercise `d = 1979-05-27`, so the test still has to pass
- update `TestTomlParser_DateTimes` (`cmd/ripc/loadbytes_v1_polifill_test.go`) from string expectations to the typed values (`toml.LocalDate{...}`, `time.Time` via `.Equal`); `get`/`walk` print `%v` and `String()` yields the same text, so display does not change
- refs: `cmd/ripc/loadbytes_v1_polifill.go` (`tomlValueOf`), `cmd/ripc/validate.go` (`validateTomlAsConfig`), `cmd/ripc/loadbytes_v1_polifill_test.go` (`TestTomlParser_DateTimes`)

# ripc: `init app` / `init logs` replace `app create` / `log init`

- `ripc init app` replaces `ripc app create`: creates the app db, applies the app schema, writes the initial encrypted config; the db must not exist yet, as today
- `ripc init logs <path>` replaces `ripc log init <path>`: creates the log db, applies the log schema, writes `log.batch.db_path`; it still needs the app db and the age key
- `ripc log` becomes a pure reader (loses both `init` and `tail` subcommands); the `app` command disappears (`create` was its only subcommand)
- update the old command names in messages and docs: `cmd/ripc/main.go` (usage example, "Please create it first using 'ripc app create'", the no-db special case keyed on `app create`), `config/config_validate.go` ("run 'ripc log init'"), `restinpieces.go` (two errors and a comment), `cmd/ripc/log_tail.go` ("run 'ripc log init <logpath>' first"), `scripts/ripdep` (warning text), `doc/ripc.md`, `doc/bootstrap.md`, `doc/post-deploy-config.md`
- refs: `cmd/ripc/main.go`, `cmd/ripc/app_command.go` (delete), `cmd/ripc/app_create.go`, `cmd/ripc/log_command.go`, `cmd/ripc/log_init.go`, tests `app_command_test.go`, `app_create_test.go`, `log_command_test.go`, `log_init_test.go`

# auth: put the iat and exp as fast check

- in `Authenticate`, read `iat` and `exp` from the fast payload and reject expired or too-old tokens before the database lookup; cheap spam filtering, not a security gate (the payload is unauthenticated until the signature check)
- the fixed 7-day `MaxTokenAge` constant is removed; the fast check should use the configured session duration (`auth_token_duration`)
- the signed post-DB validation stays as the authoritative layer
- refs: `core/auth.go` (`Authenticate`, `extractAndVerifyUserID`, `fastJwtPayload`), `crypto/jwt.go` (`NewJwt`), `crypto/jwt_validate.go` (`validateClaimIssuedAt`, `ValidateSessionClaims`), `config/config.go` (`Jwt.AuthTokenDuration`), `config/default.go`

# security: app deletes age.key after startup; restart and reload borrow the key from the plane machine

- operator keeps the canonical age.key on the local (plane) machine; the server holds it only transiently at boot, then the app unlinks it
- new config setting makes it optional; when on, restarts, reloads and ripc ops push the key, run, then verify deletion
- unlink immediately after successful config decrypt, not after N minutes — a delay only widens the window, reload can re-push the key anyway
- real gain is disk-only: file-read bugs, whole-home backups/snapshots and disk theft yield ciphertext; nothing against RCE (plaintext config lives in memory) or root
- price: no unattended recovery — `Restart=on-failure` becomes a crash loop, set `Restart=no` and rely on monitoring plus operator restart; a failed boot must delete the pushed key and alert, never leave it lying
- transport hygiene first: `cmd_cp` stages via world-readable /tmp today, fix with umask 077 plus mktemp or the transport undoes the whole idea
- unit needs a writable bind for the key path so the app can unlink it under `ProtectSystem=strict`
- refs: `config/secure.go` (key re-read per Get/Save), `restinpieces.go` (setupConfig, boot key requirement), `config/reload.go` (SIGHUP needs key), `systemd.service` (ProtectSystem=strict, Restart=on-failure), `scripts/ripdep` (cmd_cp, cmd_restart, cmd_reload, cmd_ripc, cmd_maintenance, cmd_dump), `doc/ripdep.md` (restart/reload/cp contract)

# options: every With function marks itself on the initializer

- `New` checks the app for nil (`Cache()`, `Router()`, `Notifier()`, `Logger()`) to decide whether to install a default; each `With...` should record itself on the initializer instead, e.g. `initializer.with map[<const>]struct{}`
- the decision belongs to the initializer, not the app; whether a getter is used elsewhere does not matter
- then add `WithMetrics` and `WithLimiter`
- refs: `restinpieces.go`, `options.go`

# notifier: async delivery daemon

- backends have different throughput — discord, telegram, email, etc.
- that bookkeeping is not possible with sync: for example trade for aggregation
- also a notion of importance, to be delivered right away or it can wait, same as slog => levels
- a standalone async daemon, not even dependent on restinpieces, with `go-daemon-runner` and the discord implementation first
- async also allows for listening notifiers like telegram to ask for more
- refs: `notify/`, `notify/discord/discord.go`, `go-daemon-runner`, the `# notify/discord: move rate params to the limiter section` entry below

# notify/discord: move rate params to the limiter section

- drop `config.Discord.APIRateLimit`/`APIBurst` and discord's private `rate.NewLimiter` (`notify/discord/discord.go:53-72`); use `[limiter.discord]` via `const Discord = "discord"` and take the limiter from the app factory
- seed `limiter.discord` in `config/default.go`
- refs: `notify/discord/discord.go`, `config/config.go` (`Discord`), `config/default.go`, `restinpieces.go`

# config/secure: move to pq age

- per https://words.filippo.io/right-answers/ §Asymmetric encryption: age with PQ keys (`age1pq`, MLKEM768-X25519); the C2SP age spec already carries the `mlkem768x25519` stanza
- bump `filippo.io/age` to v1.3+ (`restinpieces-backup` and `restinpieces-acme` tidy too); one key with `age-keygen -pq`, no legacy X25519 path
- `loadAndParseIdentities` requires `*age.HybridIdentity` only, `Save` encrypts to its recipient only; old rows do not open — operator re-encrypts once, `diff`/`rollback` across the change are dead
- second age spot: `config/config_validate.go` accepts only `age.ParseX25519Recipient` for `backup.s3-upload` recipients — switch to `ParseHybridRecipient`; `restinpieces-backup/s3/upload/handler.go` encrypts with that recipient and must move in the same change
- update tests and scripts that make X25519 test keys
- refs: `config/secure.go` (`loadAndParseIdentities`, `Get`, `Save`), `config/config_validate.go`, `config/backup.go` (`AgeRecipient`), `restinpieces_test.go`, `config/secure_test.go`, `config/config_validate_test.go`, `scripts/smtp_probe.go`, `scripts/s3_probe.go`, `restinpieces-backup/s3/upload/handler.go`, `restinpieces-backup/cmd/s3/download/recover/main.go`

# rate limits: possible refactor from one-per-cooldown to a general interval gate

- instant frame — elements active at the same time
- interval frame — elements inside a duration
- lifetime frame — elements ever occurring
- Yes — two different kinds of gate:
  - at the same time: `how many concurrent` — max N in flight (e.g. password verification)
  - in an interval: `how many per window` — max M in N seconds, or today's simpler form, one per cooldown bucket per account/email
- current `RateLimits` is the second, but only the one-per-bucket variant. It is not yet a general "M per interval".
- refs: `config/config.go` (`RateLimits`), `queue/queue.go` (`CoolDownBucket`), `sql/schema/app/job_queue.sql` (`idx_job_queue_incomplete`), `core/handler_auth_password_reset_otp_request.go`, `core/handler_auth_email_change_otp_request.go`, `core/handler_auth_email_verification_otp_request.go`, `queue/handlers/password_reset_otp.go`, `queue/handlers/email_change_otp.go`, `queue/handlers/email_verification_otp.go`
- the thins to dfierenciate is those limits are for someting, that something determine the implmenttion of the limit, in case of collodown is the creating of job to send emails, generalizing a concept of limit taht is usable by different use cases will prove mos rprobably not possible,
- possibility 1:

  ```toml
  [limiter.password_verify]
  frame = "instant"
  max = 8

  [limiter.password_reset_otp]
  frame = "interval"
  max = 1
  window = "1h"
  ```

  ```go
  type Limiter struct {
      Frame  string   `toml:"frame"`
      Max    int      `toml:"max"`
      Window Duration `toml:"window"`
  }
  ```

- possibility 2:

  ```toml
  [limiter.instant.password_verify]
  max = 8

  [limiter.interval.password_reset_otp]
  max = 1
  window = "1h"

  [limiter.lifetime.<label>]
  max = 1000
  ```

  ```go
  type Limiter struct {
      Instant  map[string]LimiterInstant  `toml:"instant"`
      Interval map[string]LimiterInterval `toml:"interval"`
      Lifetime map[string]LimiterLifetime `toml:"lifetime"`
  }
  ```

- if we go thei "label" route we have to make sure framwork toml paths are not deletable see ripc rm
- because implmentation is related to the kind of limits, is not generalizable, those colldown are not even to be mixed in configuration with the others, rateLimits just recevive a rename

# config.ratelimits naming is wrong this is colldown protection for email server

consideer if is possible to use genreal Limiter (not) or just naming

# core handlers: llh: log_label — separate task

- AGENTS.md logging rule: every object that takes a logger in its constructor stamps a stable label once with `logger.With("log_label", "...")`; `llh:` for handlers, and labels are permanent
- no core handler stamps one today; handlers call `a.Logger()` at the call site, so this needs App wiring (a per-handler logger) plus a label for every handler, not only the password handlers
- existing `log_label` users are daemons and jobs only: `log/daemon.go` (`lld:log_batch`), `queue/scheduler/scheduler.go` (`lld:scheduler`)
- refs: `AGENTS.md`, `core/` (all handlers), `core/app.go` (`Logger`), `log/daemon.go`, `queue/scheduler/scheduler.go`

# core/handler_auth_password_reset_otp_confirm: reset-confirm verifyErr path — evaluate later

- current: on an unusable stored password record `crypto.VerifyPassword` returns `ErrArgon2id`; the handler logs it and falls through to `UpdatePassword`, so a password reset repairs the record instead of locking the account out
- alternative: fail closed (refuse and skip the update)
- context: identity is already proven by the grant token from the email OTP step; the password comparison here is only the "same password, skip the update" optimization
- refs: `core/handler_auth_password_reset_otp_confirm.go` (`VerifyPassword`, `UpdatePassword`), `crypto/password.go` (`VerifyPassword`, `ErrArgon2id`)

# crypto/password.go: validateArgon2id message for bad thread counts — later

- current: the memory-floor check reads `parsed.threads` before the threads range checks, so a record with out-of-range threads logs the memory-floor error instead of a threads error
- decision: review nit, consequence-free — every record with bad threads is still rejected, nothing wrong is accepted, the uint32 cast cannot overflow; only the chosen error message differs
- alternative: move the two threads checks above the memory check
- refs: `crypto/password.go` (`validateArgon2id` lines 133, 145-150)

# config/config_validate.go: reject short JWT secrets at startup

- current: `validateJwt` checks secrets for empty and duplicate only; a value shorter than `crypto.MinKeyLength` (32 bytes) passes startup and fails at the first token issue with `ErrJwtInvalidSecretLength`
- change: fail fast in `validateJwt` — reject `len(secret) < crypto.MinKeyLength` for all five JWT secrets
- rationale: https://words.filippo.io/right-answers/ §Symmetric key length — 256-bit keys; app secrets are generated with `crypto.ID()` (32 random bytes)
- refs: `config/config_validate.go` (`validateJwt`, `validateJwtSecretsDistinct`), `crypto/jwt.go` (`MinKeyLength`), `config/config_validate_test.go` (short-secret fixtures need updating)

# keyed fixed-window rate limiter

keyed — the state is per identity (email, IP, user id, or one global key).
fixed-window — the timeline is cut into non-overlapping buckets, e.g. floor(now / window).
counter — for each (key, window) you store a count and compare it to max.
When N = 1, that same primitive is called a cooldown. That's the domain word, not a different mechanism.

- config: the `limiter` section splits into two branches, named after the two standard rate-limiting algorithms — `token_bucket` (the existing one: `limit`, `burst`) and `fixed_window` (this one: `prefix`, `window`, `max`).
- the names are the literature names, not invented. The Go source names the existing one itself: `golang.org/x/time/rate/rate.go` — "It implements a 'token bucket' of size b, initially full and refilled at rate r tokens per second" (https://github.com/golang/time/blob/master/rate/rate.go). The other is the "fixed window counter" algorithm listed with token bucket under standard algorithms (https://en.wikipedia.org/wiki/Rate_limiting).
- entries and the section struct:

```go
// TokenBucketEntry is one token-bucket limiter. Tokens refill at Limit and
// the bucket holds Burst of them.
type TokenBucketEntry struct {
	Limit Duration `toml:"limit" comment:"Wait between calls once the burst is gone, e.g. '2s'"`
	Burst int      `toml:"burst" comment:"Calls allowed without waiting, e.g. 5"`
}

// FixedWindowEntry is one keyed fixed-window limiter. Prefix namespaces the
// cache key; each identity gets Max calls per Window.
type FixedWindowEntry struct {
	Prefix string   `toml:"prefix" comment:"Cache key prefix, e.g. 'password_reset'"`
	Window Duration `toml:"window" comment:"Bucket width, e.g. '1h'"`
	Max    int64    `toml:"max" comment:"Calls allowed per window per identity, e.g. 1"`
}

// Limiter is the rate limiter section. Each branch is keyed by a label you
// choose and looked up by name.
type Limiter struct {
	TokenBucket map[string]TokenBucketEntry `toml:"token_bucket"`
	FixedWindow map[string]FixedWindowEntry `toml:"fixed_window"`
}
```

Yes. The limiter only needs the cache.

`Limit(label, identity)` — the caller names the label (`PasswordReset`) and hands over the identity (`email`); the prefix, window and max are read from the `limiter.fixed_window.<label>` entry, so the call site no longer builds `"password_reset:" + email` itself.

```go
func (l *Limiters) Limit(label, identity string) bool {
    e, ok := l.config.Get().Limiter.FixedWindow[label]
    if !ok {
        return false
    }
    bucket := time.Now().UnixNano() / int64(e.Window.Duration)
    cacheKey := e.Prefix + ":" + identity + ":" + strconv.FormatInt(bucket, 10)

    count, ok := l.cache.Increment(cacheKey, 1)
    if !ok {
        return false
    }
    return count <= e.Max
}
```

```go
ok := limiters.Limit(PasswordReset, email)
```

- `max = 1` is a cooldown, so this replaces the current cooldown logic: `rate_limits` (`RateLimits`: `password_reset_cooldown`, `email_change_cooldown`, `email_verification_otp_cooldown`), `queue.CoolDownBucket`, and the `job_queue` unique index that today enforces "one unfinished job per email per bucket" at OTP-job creation. The OTP request handlers would call `Limit(<label>, identity)` before enqueueing and those three go away; state then lives in the cache, not the DB, which also drops the current cooldown bug (the cooldown is broken today — see the `BUG queue` entry below).

Agreed — build the map once. `Limiter` walks both branches at construction and fills `map[label]object`, each object being the matching default (`DefaultTokenBucketLimiter` or `DefaultFixedWindowLimiter`, the latter holding the cache). After that, `app.Limiter(label)` is a plain map read — no config read, no reload, nothing extra per call.

That fits the other fact too: a new label is only useful once code names it with a const, and naming it means a deploy — so the map is built from whatever config is present at startup, and changing `window`/`max`/`burst` on an existing label needs a restart.

So the hot path is `app.Limiter(label)` → map hit → `Limit(...)`. A label code names but config lacks is the error `app.Limiter` already returns; the built-ins (`password_hashing`, and the cooldowns once they move over) are seeded so they always resolve.

Yes — with the map built at startup, `App.Limiter()` can just return the registry and the whole public surface becomes one method: `Limit(label string, identity ...string) bool`. Token bucket ignores the identity; fixed window reads it (exactly one, else panic). No per-label accessor, no assertion at the call site, and `Limit(PasswordHashing)` / `Limit(PasswordReset, email)` read the same.

Right, and it hangs together: the token bucket keeps per-label state (its own tokens), so it needs one object per label; the fixed window keeps no state in the object (the count lives in the cache), so a single object serves every fixed-window label. That's why `FixedWindow` is just `map[label]struct{}` — a marker of which labels are that kind — `TokenBucket` is `map[label]object` — the per-label token buckets — and `FW` is the one shared object.

On `Limit(label, identity...)`: if `label` is in `FixedWindow`, call `FW` with `(label, identity)` —

```go
type TokenBucketLimiter interface { Limit() bool }                // per label, holds its own tokens
type FixedWindowLimiter interface { Limit(identity string) bool } // one shared, cache-backed

type Limiter struct {
	FixedWindow map[string]struct{}           // labels that are fixed-window
	TokenBucket map[string]TokenBucketLimiter // label -> its own token bucket
	FW          FixedWindowLimiter            // the one shared fixed-window object
}

func (l *Limiter) Limit(label string, identity ...string) bool
```

following the backup types (`backup-online`, `backup-vacuum`, …) — it would become `ripc scaffold limiter-token-bucket <key>` and `ripc scaffold limiter-fixed-window <key>`

- increment must be not in cache interface, it is not supported by go caches like ristretto

Yes. That is the clean refactor.

`Get` and `SetWithTTL` become thin lock wrappers. `Increment` takes the lock once and calls the unlocked helpers.

```
Get(key):
    lock
    defer unlock
    return get(key)

SetWithTTL(key, value, ttl):
    if ttl < 0:
        return false
    lock
    defer unlock
    return set(key, value, ttl)

Increment(key, delta):
    lock
    defer unlock

    current, found = get(key)
    if not found:
        set(key, delta, 0)
        return delta, true

    if current is not int64:
        return 0, false

    count = current + delta
    set(key, count, 0)
    return count, true
```

`set` takes `ttl = 0` for counters, so they never expire. The bucket in the key is what resets the count.

```
Increment(key, delta):            // func (c *Default[K, V]) Increment(key K, delta uint64) (uint64, bool)
    lock
    defer unlock

    n, ok = c.index[key]
    if not ok:                                       // no counter yet
        n = c.alloc()
        if n == -1:                                  // full: evict the LRU tail
            tail = c.tail
            c.unlink(tail)
            delete(c.index, c.nodes[tail].key)
            c.dealloc(tail)
            n = c.alloc()
        c.nodes[n].key = key
        c.nodes[n].value = any(delta).(V)            // box delta, assert back to V
        c.nodes[n].expiration = 0
        c.index[key] = n
        c.linkToHead(n)
        return delta, true

    current, ok = any(c.nodes[n].value).(uint64)     // box V, assert to uint64
    if not ok:
        return 0, false

    count = current + delta
    c.nodes[n].value = any(count).(V)                // box count, assert back to V
    if n != c.head:
        c.unlink(n)
        c.linkToHead(n)
    return count, true
```

# BUG queue: we added WHERE status != 'completed' and broke the cooldown

- the unique index only covers pending, processing and failed rows; once the scheduler marks the email job completed, a new request for the same email and bucket inserts another job and sends another email
- the rule is "one unfinished job per key per bucket", not "one event per key per window"
- the cooldown is broken today. The `# keyed fixed-window rate limiter` design replaces it with a cache-backed `fixed_window` entry (`max = 1`), removing this bug together with `rate_limits`, `CoolDownBucket` and the unique index
- refs: `sql/schema/app/job_queue.sql` (`idx_job_queue_incomplete`), `db/databasesql/queue.go` (`StmtInsertJob`, `MarkCompleted`), `queue/scheduler/scheduler.go`, `queue/queue.go` (`CoolDownBucket`), `core/handler_auth_password_reset_otp_request.go`, `core/handler_auth_email_change_otp_request.go`, `core/handler_auth_email_verification_otp_request.go`

# cache: make a smoke test with struct and no struct for memory vs time

- one smoke test comparing a struct key against the composed string key, measuring memory and time
- refs: `cache/default.go` (`node`, `index`), `cache/benchmark_test.go`

# limiter: limiter uses Default, no interface

- refs: `core/limiter.go`, `cache/default.go`

# limiter: fixed-window cache key — one allocation, no buffer pool

- the sketch in `# keyed fixed-window rate limiter` builds `cacheKey := e.Prefix + ":" + identity + ":" + strconv.FormatInt(bucket, 10)`; `FormatInt` plus the concatenation is two allocations per call
- preferred: one local `strings.Builder` with `Grow`, append the prefix, identity and bucket with `strconv.AppendUint` into a stack `[20]byte`, then `String()`. One allocation, and the cache owns a stable string
- do not pool `[]byte` or `bytes.Buffer`: `string(buf)` still copies and allocates, so the pool does not remove the allocation that matters and adds `Get`/`Put` overhead
- do not pool `strings.Builder`: `String()` aliases the builder buffer, and the cache stores the key in the map and node; reusing the buffer would corrupt the stored keys
- never use `unsafe.String` over a pooled buffer for the same reason
- if a profile shows the key build dominating: add a `[]byte` lookup path (`m[string(b)]` map lookups do not allocate; allocate only on a miss) or a purpose-built window structure
- measure with `-benchmem`
- refs: `doc/TODO.md` (`# keyed fixed-window rate limiter`), `cache/increment.go`, `core/limiter.go`, `cache/benchmark_test.go`
