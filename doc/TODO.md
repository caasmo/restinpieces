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

## OAuth2 providers map key refactor

Map keys currently carry domain logic (e.g. `"google"` is the provider identifier). Refactor so keys are arbitrary labels and `OAuth2Provider.Name` holds the identifier. After refactor, `cfg.OAuth2Providers[req.Provider]` becomes a lookup by `Name` field.

See AGENTS.md "Config: map key rules".

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
- add prometheus.
- s3 integration
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
- refs: `core/handler_auth_register_password.go`, `core/handler_auth_login_password.go`, `core/prerouter/`, `restinpieces-js-sdk`

# json: update to go json v2

- migrate from `encoding/json` to `encoding/json/v2` (`go 1.25.0` in `go.mod`)
- refs: `core/auth.go`, `db/types.go`, `db/databasesql/queue.go`, `oauth2/oauth2.go`, `log/daemon.go`, `notify/discord/discord.go`, `queue/handlers/`

# config: remove the exported ValidateBackup

- `ValidateBackup` is exported only so the restinpieces-backup daemons can validate their `[backup]` section on their own
- the framework owns the shape and its validation, so the section validator should not be part of the public API
- decide the replacement before changing the call sites: validate through the framework's `Validate`, or through a loader that validates the section
- refs: `config/config_validate.go`, `config/backup.go`, `restinpieces-backup/cmd/vacuum/daemon/main.go`, `restinpieces-backup/sqlitersync/origin/daemon.go`

# ripc log tail: create filter for tail as argument it matches message field

- `ripc log tail [message]` filters by the `message` column (exact match, e.g. `http_request`)
- refs: `cmd/ripc/log_command.go`, `cmd/ripc/log_tail.go`, `cmd/ripc/sql.go`, `sql/schema/log/logs.sql`

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

# ripc: remove documented as only for maps or slices

- new `ripc remove <path>` drops one item from a config map or slice by dot-path (e.g. `ripc remove backup.online.logs-online`); scalar keys are refused
- mirrors `add`, which only touches its registry of collections — scope comes from the registry, not from extra rules
- the industry pair is add/remove (Azure CLI guidelines, PowerShell approved verbs pair Add with Remove and forbid Delete)
- refs: `cmd/ripc/add.go` (`addFuncs` registry precedent), `cmd/ripc/set.go` (path handling precedent), `cmd/ripc/main.go` (command dispatch)

# ripc: add a helper for the tree type check

- the `value.(*toml.Tree)` assertion telling tables apart from plain values is copied in every command instead of living in one place
- pull it out into a shared helper so `remove` (and whatever comes next) reuses it instead of inlining a third copy
- refs: `cmd/ripc/paths.go` (`listTomlPathsRecursive`), `cmd/ripc/get.go` (`listTomlPathsWithValuesRecursive`), `cmd/ripc/add_block_user_agent.go` and `cmd/ripc/add_block_host.go` (slice assertions)

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

