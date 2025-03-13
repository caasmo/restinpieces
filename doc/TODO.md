### TODOs


- refresh jwt
- generate new jwt register 
- jwt invalidation go tests
- integration sh test post invalidation jwt 
- User schema, remove tokenKey
- jwt signing key invalidation
- test requestverification: test also insertion in queue, now only mock nil.
- more bash test all endpoints
- test moe all to dbsetup
- /impl backend job to verification email
    - reads from job_queue 
    - generates token
    - put in user tokenkey????? no is jwt
    - sends email with mailyak
    - status processinnf in job, each job has steps laststep issaved in jobqueue steps are label with humman code explanation
    - /confirm-verification endpoint 
        - returns 204, or error 400
        - get generated token from db, compares to token from request
- code review jwt tests
- zombiezen, impl pool with timeout, split in files. 
- zombiezen, crawshaw, use stmp.step, handling of conn with setinterrupt and timeout
- httprouter params to servemux $ 
- tls
- signal, add baseContext
- add logging
- hardening: add headers CORS, etc
- add toml conf and config struct, add struct to app, router, cache
- document design in doc. why all decision.
- frontend integration with fs embed 
- integrate 3 party middleware
- add prometheus.
- s3 integration
- proper error handling from sqlitex, timeouts.
- document performance read/write 
- rand source in app. performacen rand
- make command line to copy files and perform changes in the codes based on preferences. maybe using generate
- More backends: badger and boldb
- the command (maybe based on configuration) creates dir, copy only needed packages and inserts custom code pa

### done

- ~~timeouts in server~~
- ~~move gratefull shutdown to server.go~~
- ~~integrate, benchmark ristretto~~
- ~~signal to stop handling~~
- ~~write normal pool insert~~
- ~~encapsulate router, maybe later interface~~
- ~~context/application package~~
- ~~context request~~
- ~~model~~
- ~~sqllite with cranshaw~~
- ~~remove reference to "github.com/julienschmidt/httprouter" in handlers. To
  know the key in the context, we should not need the router. After router
  init, find the context key and pass to the app. or just harcoded conf in toml
  Or just used explicite params.~~
- make sh files inside repo for testing with curl different parst
    - generate jwt sh maybe also signing method
        - generate token 1`
    - testendpoint send curl with different tokens. 
    - provide file in docs to test all of them combined
