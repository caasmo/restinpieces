### TODOs

- ripc: all mutating config subcommands should validate before Save (fail-fast on `set`)
    - `ripc set backup.sqlite-rsync.listen_addr bad` currently reports `Successfully set` then fails on next `HUP` (`backup config validation failed: ... missing port`). Every mutating subcommand (`set`, `scaffold`, `migrate`, `save`) should `toml/v2.Unmarshal` the updated bytes into `config.Config` and call `config.Validate` before `SecureStore.Save`.
    - Keeps single source of truth (`config.Validate` / `ValidateBackup`), gives immediate operator feedback, avoids persisting invalid rows.
    - Split the `v2` unmarshal into a separate file (`validate_set.go`) to respect `AGENTS.md` “Never import both v1 and v2 in the same file” — `set.go` stays `v1`-only (`tree.Has/Set`), helper is `v2`-only.
    - Note: `ValidateBackup`’s `isFile`/`isDir` checks are CWD-dependent; for local `RIPC_DB` (same host/CWD as app, the normal `systemd` case) they are accurate and should stay. For a future remote-CLI case they are conservative (rejecting a non-existent file early is still better than persisting it).
    - Repro: `ripc set backup.sqlite-rsync.listen_addr bad` → `ripc get backup` shows `bad` → `kill -HUP` → `Configuration reload failed: sqlite-rsync.listen_addr "bad" is not a valid host:port` (fixed by validating on `set`).

- config: delete config.Provider (no wrapper) — the current-config box is stdlib atomic.Pointer[config.Config]; core.App owns it via a ConfigPointer() getter, Reload Stores into it, every consumer reads Load() (see brainstorm-restinpieces-daemons.md Q23)
    - ConfigPointer() is for external wiring (daemons, jobs); Config() stays the canonical read for internal consumers (handlers, middleware)
    - End-state framework changes — no migration noise: the box is atomic.Pointer[config.Config], Provider is deleted, every consumer reads Load()
        - (a) config.Provider is deleted — the atomic.Value holder was the misplaced construct; the config package keeps only domain types, defaults and validation. The current-config box is the stdlib *atomic.Pointer[config.Config] everywhere in the framework.
        - (b) core.App owns the box: it holds *atomic.Pointer[config.Config], exposes it via a ConfigPointer() getter, and Config() becomes box.Load().
        - (c) config.Reload re-publishes into the box — signature takes *atomic.Pointer[config.Config], SIGHUP path does parse → validate → Store.
        - (d) Every framework consumer switches to the box: server, scheduler, log daemon, batch handler, mail, auth — all take *atomic.Pointer[config.Config] and read Load() at each use. One box type, one read op, framework-wide.
- handler_fs has to return framwork errors http.error does things to header
    - Error deletes the Content-Length header, sets Content-Type to
     “text/plain; charset=utf-8”, and sets X-Content-Type-Options to “nosniff”. This
    configures the header properly for the error message, in case the caller had
    set it up expecting a successful output.
    core/handler_fs.go
 99:                     http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
153:                    http.Error(w, "internal server error", http.StatusInternalServerError)

core/prerouter/block_oversized_request.go
 90:                     http.Error(w, http.StatusText(http.StatusRequestURITooLong), http.StatusRequestURITooLong)
- core/prerouter/block_oversized_request.go the same for http.Error, has our writeError do we implment it

- dump does not work for scope, shoudl check format, is crap!!! harcoded framwork! must be explicite!!!!
- scope should be restinpieces or rip let apps have app scope
    - config.ScopeApplication is bad ScopeRestinpieces
- ripc should have sane defaults and env variables
- compare ripc structure with best practices last command Put uptodate
- headers, export 
    The Common Element
    Yes, there is one — and it's already in the framework. It's SetHeaders(w, ...map[string]string).
    The variadic merge is the abstraction. Later maps override earlier ones. That's composability.
    The framework's current problem isn't the function — it's that the named sets are private. The moment you export them as granular building blocks, every use case you listed is solved without adding any new concepts:
    go// restinpieces/core — exported, documented, tested building blocks
    var HeadersCacheImmutable   = map[string]string{ "Cache-Control": "public, max-age=31536000, immutable" }
    var HeadersCacheHTML        = map[string]string{ "Cache-Control": "public, no-cache" }
    var HeadersCSPStrict        = map[string]string{ "Content-Security-Policy": "default-src 'self'; frame-ancestors 'none'" }
    var HeadersCSPPermissive    = map[string]string{ "Content-Security-Policy": "default-src 'self'; script-src 'self' 'unsafe-inline'..." }
    var HeadersNoSniff          = map[string]string{ "X-Content-Type-Options": "nosniff" }
    var HeadersReferrerStrict   = map[string]string{ "Referrer-Policy": "strict-origin-when-cross-origin" }
    var HeadersHSTS             = map[string]string{ "Strict-Transport-Security": "max-age=63072000; includeSubDomains" }
    Each use case you listed becomes explicit composition in the app's own middleware:
    go// want permissive CSP?
    core.SetHeaders(w, core.HeadersCacheHTML, core.HeadersNoSniff, core.HeadersCSPPermissive)

    // want custom cache?
    core.SetHeaders(w, myCustomCache, core.HeadersNoSniff, core.HeadersCSPStrict)

    // want sha256 hash?
    core.SetHeaders(w, core.HeadersCacheHTML, core.HeadersNoSniff, myCSPWithHash)
    The framework can still ship StaticHeadersMiddleware as a convenience
    default — just a thin wrapper that calls SetHeaders with a pre-chosen
    composition. Users who are happy with opinionated defaults don't write
    anything. Users who aren't, write their own middleware using the building
    blocks, without reimplementing the primitives.
- config.provider generic?
    - users can use it to create reload of its config
    - but is just 15 lines they can do their own by copying
- expose mailer and make a developetment one
- simple development logger
- /link-oauth2 and /link-password handlers sdk, authenticated yes
- all no auth handlers security enumeration time attack
- ClaimUidMac is added to many jwt, do we need all, Check!!!!!!
- precomputed Errors and Ok shoudl be exportable
- bug handler_auth_login_password.go
    receive 503, maybe because no cooldown in the config yet for 
    but only happens ocasianally, not always
- register and login handler, those are email, 
    - in register remove confirm???
- register login handler form errors with fields etc 
- pow middleware, you can put there the paths -> config
    present html with javscript, like cloudflare
    can adjust cost in the config
- validation revamp
    - move to own package
    - use should be able to inject map of passwords
    - user could set own validator PART, ie the email or password validation
        - functions? instead 
        - added script for generation, document an addon? like the others? 
        - no configuration, do your own validator email function
        - paswords the customBreachPasswords, app.Validator().AddPasswords(typed map)  
        - we leverage configstore, other scope -> ripc accepts list of words separated by \n
            - see other scopes like litestream
- resend otp
    - we need a little more sofisticated emial rate limit
        - allow not one per period
        - do not overcomplicated
        - hidden payload in 
        - err = a.DbQueue().InsertJob(job)
            naming is bad, it shoudl be unique or something
            we could select write in a transaction meh
            exponential too complex
            4 per 5 min,    
            1 per minute 5 hour 2 periods 
- sdk shoudl wrap two calls, /register and /request-otp  some form of chain
   - we can not let user code do that??? we must i think 
   - how to make eaiseier for the user
- TODO-Email-verification-in-registration-workflow.md
    - TODO-sigup-login-otp-refactor.md the end is the corroboration

    - **`authenticated == verified`. Full stop.**

    - authnticate() add verified throw error needs_email_otp_verification 
    - login with email, if not verified throw error needs_email_otp_verification 
        - yes, separeated, we could also atach 2FA in the login with the same system returning needs_2fa_auth
        - js should check that and present a otp component with otp shadcn, execute call to resend otp
        remember starttranstion (this is react thing)
    - /register puts name email in db, returns 202 needs_email_otp_verification after writing in the queue 
        -sdk upon code needs_email)otp present a otp 

        httpHTTP/1.1 202 Accepted
        Content-Type: application/json

        {
          "status": "pending_verification",
            "message": "Registration received. Please verify your email to continue.",
              "next": "/verify-email"
              }
              202 is your friend here. It signals success without completion, which maps perfectly to "we got you, now go check your email."

    | `RegisterWithPasswordHandler` | No token issued — user told to check inbox |
    | `AuthWithPasswordHandler` | Rejects before token generation (`errorEmailNotVerified`) |
    | `Authenticate` (auth.go) | Rejects valid tokens from unverified users (defense in depth) |
    | `RequestEmailVerificationHandler` | Becomes unauthenticated — fetches user by email directly |

- deprecate email verification for register
- payload and payloadextra -> maybe create struct to be more clear and explicit about uniqueness
    PayloadBuilder.BuildPayloadUnique, BuildPayloadExtra???
- PayloadEmailVerification in db/types? remove
- https://github.com/ai-robots-txt/ai.robots.txt
    plugin for ripdep to add 
    plugin to ripc
- DFA regexp compression https://github.com/coregx/coregex?tab=readme-ov-file
- hyperscan,  Aho-Corasick for literal substrings ⭐ most practical for user agents
- "github.com/cloudflare/ahocorasick"

- subscribe button banner/
    - https://ideasai.com/ 
    - has a banner (like cokie) where you give email 
        join 98000 person to receive a 
        get (per javascript)
            Check your email and click the link to confirm your email!
- ripdep undeploy unistall is ugly, rethink
- ripdep backup is ugly, rethink
- ripc rotate -agekeynew  

- BUG
    - if no scope default is applciation, is that right?
        - empty string instead?

- BUG
    - if there is a error in initializing (restinpieces) and we are alrady activated the batch handler logger, it will not Flush 
   on error,  
        - last error should flush all log message 
        - or at least do not use the the app.Logger in entry points
        - bug ocurred in restinpieces-litestream
            - we do not have litestream.yml uet in sqlite
            - litestream init fail with error
            - we had in case of error app.Logger().Error("failed to init Litestream", "error", err)
            - that message is batched in the default logger
            - we do exit(1), no log, message in terminal or sqlite3 
            - changing to slog.Error: 2025/12/16 16:51:17 ERROR failed to init litestream error="failed to load Litestream config from DB: securestore: decrypt failed: failed to read header: parsing age header: failed to read intro: EOF"

            - DO NOT use app.Logger for restinpieces.New(), document
   - maybe activate the logger only after initalization?

- integrate local backup in main framework
    - handler receives conn to app file? no better create two more. ones for source one for destiny they are harcoded zombiezen
        - we can not use app.DBQueue etc.
    - about command restinpieces wiht serve and ripc integration create etc
        - no restinpieces is meant to be always extended: is a framwork
            - there is no restinpices binary
            - we can call ripc, and add create command to reduce 
            - only one binary con configuration and creation
            - rename ripc, rename restinpieces, must be example. 
            - commetn is meant to be extended.
    - about inserting backup job
        in ripc job-add <template>? it must be always a handler, handler could decribe the Job.
        job-list id recurrent payload prefix payloadextra prefix
        job-deactivate id for recurrent just modify flag recurrent
        job-interval id 24h
- maintenance: mimetype decides output
- https://github.com/jellydator/ttlcache
- simple ttl map instead of ristretto  https://stackoverflow.com/questions/25484122/map-with-ttl-option-in-go
- alternative litestream workflow in daemon.
	- why not a simple script ssh hosted in client or machine, using just litestream binary
	- ssh ltbackupme   
		- ltbackupme uses litestream binary to create db file in tmp
		- it uses scp to bring the file
		- or just local litestream that downloads from s3
- https://raw.githubusercontent.com/caddyserver/caddy/refs/heads/master/cmd/caddy/setcap.sh
- good enough release
    - superuser workflows
		- scripts
	    - workflow for recovery, 
	- all shell test 
	- unit test
	- code review
    - dunctional tests
    - documentation
	    - basic framework use examples repo, with examples of features.
	- pretty logging
- logger, have a text logger for startup before app logger, and for shutdown. pass to the app.logger and use as default?
    - logdb must be propely wal etc
- default logger db interface?
- shoutdown, with context in log handler, is better. But not enough. change logger to standard/ do not shoutdown log daemon concurrently
- mailer default local
    - no external smtp server
- test functional uablock
- mailer interface for app/server, is for server though
- sdk visibility, own route
- create-app shoudl create age key.
- disable standard routes
- is a framework, clear workflow  -> examples repo. od use of the features
- nocache? what about BlockIp.
- notifications
    - slog? https://github.com/betrayy/slog-discord
- script insert-job. --type 
- config reload
- race detection
- password reset if no password ie oauth2 user => no only register with email
- verify email for oauth -> yes verifed
- corfirmation, spam sending the same right jwt 
- endpointsw discovery has no update each time.
- assets integrity, bundler 
- confirmation endpoints spam attacks
	- attacker with valid email token (1 hour) can spam until token expiration
	- this is jwt attack, 
	- damage is 1 read 1 idempotent write
	- for confirmation and expensice path, maybe hash the page (or paht) in cache with ttl, already requested try in a minutes
- request change endpoints spam attacks TODO
- request email verification must be logged
- cache and other headers from assets use a middlware for api we have a map that we appli in response
    - try to be consistent
    - 'static' form html, js, css ... and api for dinamic 
    - gzip header moev to response_headers
- document magic numbers of sketch. move it to new package, configuration
- verify addresses paths shoudl be random or pseudo random?
- revamp shell tests.
- in process litestream 
- document middleware politic, if you have to write in the context, you shoudl not be a middleware.
	- the first middleare post serverHttp code is the last observer.
- superuser? just ssh?
- metrics
	https://github.com/prometheus/client_golang
- sheurl hadcoded https. should be configurable if srver http under proxy TLS like cloudflare
- downtime page schedule, all routes to, lock db ...
- error in trhe sequnce of step f ex register can let inconsitent state, ex
    - error after inserting job, we have user in db but no varification 
- generate new jwt register 
- jwt invalidation go tests
- integration sh test post invalidation jwt 
- test requestverification: test also insertion in queue, now only mock nil.
- more bash test all endpoints
- code review jwt tests
- httprouter params to servemux $ 
- hardening: add headers CORS, etc
- document design in doc. why all decision.
- document performance read/write 
- the command (maybe based on configuration) creates dir, copy only needed packages and inserts custom code pa
- minify html, 5% space. if we already have gzip
    - https://github.com/tdewolff/minify?tab=readme-ov-file#html 
    - https://github.com/privatenumber/minification-benchmarks?tab=readme-ov-file#%EF%B8%8F-minifier-showdown

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
- startup: 
    - func of type Option func(rip), or no options at all 
    - WithCache() crete empty app and set or apply
    -WithDaemon create empty server and apply
    -WithJobHandler 
    - WithMetrics, will make handler, middleware and conf
    - we can still retiurn app and server. 
    - jsut not options
- NewWithConfig(restinpieces.Config{
- updatebenchmark: to own paclkage resuse modernc and 
- modernc?
- add prometheus.
- s3 integration
- cache alternative syncMap, no garbage collection, noOP
- propably multidomain
- ETag or Last-Modified: Enables efficient cache validation for performance. -> no: user
    - no we are talking about html.
    - at most a weak etag like deploy tag
    - maybe max-age 1 hour in cache control
    - opinionaated, user can make its own
- block ua: cache db,  
- block jwt: cache db,  
- block referrer
- rand source in app. performacen rand

# secureStore: GetConfig nil content causes misleading "decrypt failed" error

- `zombiezen.GetConfig` returns `nil, "", nil` when no rows match scope (ResultFunc never called)
- `secureStoreAge.Get` passes nil `encrypted` to `age.Decrypt` which fails with "failed to parse age header"
- user sees "decrypt failed" but real problem is "no configuration found for scope X"
- affected: dump, get, paths, diff, reload — any command calling `secureStore.Get()`
- ref: `config/secure.go:81-107`, `db/zombiezen/config.go:14-48`

# secure_test: mock GetConfig returns error for not-found but real zombiezen returns nil

- `config/secure_test.go:43-47` mock returns `errors.New("not found")` for missing scope
- real `db/zombiezen/config.go` returns `nil, "", nil` (no error) for same case
- the nil-content → decrypt code path is never exercised in the secure store tests
- ref: `config/secure_test.go:32-69`, `db/zombiezen/config_test.go:61-74`

# NewDefaultConfig: should be deterministic, secrets generated by ripc

- `config.NewDefaultConfig()` calls `crypto.RandomString()` for 6 secret fields
  (auth secret, password reset secret, email change OTP secret, verification OTP
  secret, OAuth2 state secret + token durations)
- this makes every call non-deterministic: server startup, `--runtime` dump,
  test scaffolding all get different random strings
- proposal: `NewDefaultConfig()` returns zero/empty secrets. ripc is responsible
  for generating them:
  - `ripc app create` already saves the full default config with generated secrets
  - `ripc config init` ditto
  - maybe add `ripc config generate-secrets` or similar to regenerate in-place
- no ripc command currently exists to generate secrets independently
- ref: `config/default.go:25-34`, `cmd/ripc/app_create_command.go`, `cmd/ripc/init_command.go`

# server: unstarted daemons are stopped after a startup failure

- `Server.Run` (server/server.go): when a daemon fails to start, the start loop breaks but the shutdown loop still stops **all** registered daemons, including those never started
- their `Stop` waits on a `ShutdownDone` that never closes, burning the full `ShutdownGracefulTimeout` (default 15s) and returning a spurious "daemon failed to stop gracefully: context deadline exceeded" error
- fix: track started daemons (append after each successful `Start()`) and stop only those — already fixed in the go-daemon-runner extraction (impl-daemon-runner.md Phase 3, `startedDaemons`)
- ref: `server/server.go:184-194` (start loop), `server/server.go:262-276` (shutdown loop)

# logs schema: random TEXT primary key scatters inserts and bloats sqlite-rsync syncs

- the `logs` table uses a random TEXT primary key: `id TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL`
- nothing writes it: `InsertBatch` (`db/zombiezen/log.go`) inserts only `(level, message, data, created)`; the id is filled purely by the schema default
- nothing reads it: no SELECT, no reference, no consumer anywhere in restinpieces or writeplay; the tailsqlitelogs reader selects `created, level, message, data` — never `id`. (The `users` table uses the same random id for a reason — external users need a stable, unguessable identifier. `logs` has no such need.)
- a `TEXT PRIMARY KEY` in SQLite creates an implicit unique index (`sqlite_autoindex_logs_1`), so every insert writes into **three** B-trees: the PK autoindex (random key → random leaf), `idx_logs_level`, and `idx_logs_message`
- the random key scatters each row to a different leaf page and causes page splits; measured: 5 log rows ≈ 14–21 `page_updates` on the next sqlite-rsync replica sync, versus ~1 page for a quiet minute
- proposal: drop the random `id` from `logs`; the table already has an implicit sequential `rowid`, so inserts append at the tail of one B-tree. Either:
  - drop `id` entirely: `CREATE TABLE IF NOT EXISTS logs (level INTEGER DEFAULT 0 NOT NULL, message TEXT DEFAULT "" NOT NULL, data JSON DEFAULT "{}" NOT NULL, created TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ')) NOT NULL)`
  - or use `INTEGER PRIMARY KEY` (rowid alias) for an explicit conventional key — still sequential/append-only, no autoindex
- schema change in `migrations/schema/log/logs.sql`, affects the app DB migration, not daemon code
- ref: `migrations/schema/log/logs.sql:6`, `db/zombiezen/log.go:55`

# block_ip: buckets + TTL redundant — 40ns vs 90ns

Buckets are pointless with TTL.

With `SetWithTTL + no buckets` = precise 3m, but `Get` 90ns (TTL branch + `time.Now()`)
With `Set + buckets at 3m` = coarse 3m, but `Get` 40ns (no TTL, `expiration==0` skips `time.Now()`), fastest under attack
Current `3600s bucket + 3m TTL` is worst of both

# cache: rotating cursor sweep (W/K) — reclaim expired never-read entries

- `cache/default.go` is preallocated LRU (`Get` lazy-expires, `SetWithTTL` evicts LRU tail). Expired never-read entries waste effective capacity.
- TODO: inline sweep, no goroutine, bounded `W/K`. `W = window` nodes per sweep, `K = every` K writes — e.g. `W=64, K=64` → `W/K=1` check/write, full pass every `maxEntries` writes, one constant for all levels. Hook `sweepIfDue()` at top of `SetWithTTL` (under lock); `sweep()` walks `W` slots from `cursor` (wraps), skips free, frees `expiration !=0 && fastNow()>expiration`.
- Live-vs-free still open: `used bool` vs `expiration==0` sentinel vs map lookup (Q41/Q42, you choose). `cost` not involved (Q39).
- Refs: `cache/default.go:77-79`, `192-193`, `brainstorm-remove-ristretto.md` Q10/Q22/Q33/Q40.

# BlockRequestBody maybe we shoudl check less is too much checks

# server: implement MaxHeaderBytes

- add `Server.MaxHeaderBytes int` `toml:"max_header_bytes"` in `config/config.go`
- default `1<<20` in `config/default.go`, validate in `config/config_validate.go`, wire to `http.Server` in `server/server.go`
- ref: `config/config.go:Server`, `server/server.go:123`

### done

# sqlite driver: enumerate the files to substitute zombiezen with modernc

- `db/zombiezen/` — the driver wrapper package to replace with a modernc-backed wrapper (`db.go`, `config.go`, `users.go`, `queue.go`, `queue_admin.go`, `log.go`, `pool.go`, `conn.go` and their `*_test.go`)
- `db/zombiezen/db.go`, `db/zombiezen/log.go` — package entry points `New`, `NewLog`
- `sqlite_zombiezen.go` — root public constructor API (`NewZombiezenPool`, `NewZombiezenPerformancePool`, `NewZombiezenConn`, `WithZombiezenPool`), still returns raw zombiezen types
- `restinpieces.go` — `newLog` uses `zombiezen.NewLog`
- `cmd/ripc/sql.go` — holds a `*sqlitex.Pool` and imports the wrapper
- `cmd/ripc/sql_helpers_test.go` — test pool helper
- `go.mod` — the `zombiezen.com/go/sqlite` module dependency to be swapped for `modernc.org/sqlite`
