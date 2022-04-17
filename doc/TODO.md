### TODOs

- timeouts in server 
- document design
- fs, embed for html endpoint.
- s3 integration
- tls
- integrate, benchmark ristretto
- integrate 3 party middleware
- proper error handling from sqlitex, timeouts.
- rand source in app. performacen rand
- document performance read/write 
- signal to stop handling
- make command line to copy files and perform changes in the codes based on preferences. maybe using generate
- More backends: badger and boldb
- add toml conf and config struct, add struct to app
- the command (maybe based on configuration) creates dir, copy only needed packages and inserts custom code pa

### done

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
