### TODOs


- all no auth handlers securitu enumeration time attack
- ClaimUidMac is added to many jwt, do we need all, Check!!!!!!
- precomputed Errors and Ok shoudl be exportable
- check conflict handling in createuserforaouth2
    - do we still need it?
    - state we do not support signup with password after google???? document
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
    - passwords, NIST dice space allowed!!!! no trim 
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
- create a middleware prerouter for error capabilitymitmatch
    - present examples of other middleware 
    - show restinpieces.js
    - show response_auth, precomputesd we do not need data
    - discuss naming of precomputed 
- sdk shoudl wrap two calls, /register and /request-otp  some form of chain
   - we can not let user code do that??? we must i think 
   - how to make eaiseier for the user
- TODO-Email-verification-in-registration-workflow.md
    - TODO-sigup-login-otp-refactor.md the end is the corroboration!!!

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
- test for otp. follow impl-otp.md, create test skill. 
    - from 78,2 to 75.2!!!!
- dbAuth, verifyEmail naming is bad => Setverification or something
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

### done

