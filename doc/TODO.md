### TODOs


- handling of flow for oauth2/passwords:
	- Implemetation Conclusions
		- add field externalAuth: string. if string contains oauth2, that means
		  this row email was used in combination with oauth2 of some/many provider.
			- structure of field to allow for future  mfa:
				- no mfa: auth1,auth2 (password auth excluded) where , has semantic of AND, presence of "," means no mfa
				- mfa: auth1:auth2, presence of : means mfa, 
		- name of field shoudl suggest alternativeAuth, meaning basic pasword auth is excluded here, 
			- externalAuth, OTP per email, oauth2, OTP app. all require something external to the app. 
		- User struct should have hasMfa method, mfaAuthfirst, mfaAuth2Second, hasAuthWithPassword, hasAuthWithOauth2 
			- also method AuthMethods() contain password, by lookin presence of password
		- we do not want to write redundant info in that field, existence of password is enough to know auth with password
			
	- we could implment in the future mfa, do not forget with the following considerations.
	- a user can log with many oauth2 providers IF they all are realted to one email account
	- email is always UNIQUE for the user, we do not allow two emails, 
		- two emails would be a nightmare, email is the channel of communication with the user.
		- ex login with email1 and password, and oauth2 provider that ahs other email should not be possible
	- oauth is passwordless, we should maintain that in the Users table, password field is the key field to controls the endpoints interaction
	- jwt signing key should be possible without password
		- make tests 
	- if user has password login and uses that same email with a oauth2, that is posssible, we just add auth field oauth2
	- if user request /request-email-change, whithout password,  it should be denied with error indicating not possible
		- we could say request a request-password-reset if your intention is to login in the future with password
		- lets say user uses oauth2 google, want to get rid of that:
			- only solution is request-password-reset, 
				- the UI can see user has no password and change text of
				  request-password-reset, "Allow also login with password"
	- if user request /request-verification whitout password, we should show
	  alreadu validated, because we only allow exterval auth methods that produce
	  verified email
	- if user request request-password-reset whithout password, we allow it, it is the way to possibly remove the oauth2 provider
		- there must be a way to transition from oauth2 to password based auth
		- this produce a user that can login with oauth2 and with password 
		- if that user after that change the email, than it is possible, but
		  that obviosly invalidate the oauth2 one. Login with oauth2 will create another user. Could be a surprise for the user.
				- with the externalAuth set to oauth2 we say the user "you are
				about to change the email but that will invalidate login with oauth2 providers asssociated with that email"
					- we do not list the providers, it could be many if user hast same email by many.
	- if user uses many oauth2 providers (with same email) we do not update avatar, name etc
			- after first login/register the name avatar is appropieated by the app. in the future can or can not be edited
	- we could allow delete password, only having alternative login nmethod like oauth2.
	- double pasword, oauth2?
		- only through request-password-reset 
	- when first auth-with-oauth2, ie register with oauth2, we do not allow it if email not validated by oauth2 provider
		- oauth2 registration always produce verified true
	- db methods shoudl not make validation, handlers should make it, like create user with verified and no password.

- mapping go struct <=> sqlite queries (insert queries, only those use default)
	- using crawshaw Exec only allow for determined number of placeholder
	- example a insert for createUser, we want sqlite to write the default created, updated. but the go struct is empty
		- each method (query) should write all values in go 
		- each method determine the list of arguments, which argumetn will update, and the number of placeholders.
			- createUser for oauth2
				- created and updated do not write

			- createUser for password
				- created and updated do not write
	- i think the idea is
		- check the schema definition, look for DEFAULT in schema that are not zero
			- if schema has a default it means it wants to write it, let it
		- for each query, method, determine which values come from go, and which ones are writen by the db
		- it should check all posssible callers of the method
			- oauth2 creation of user has avatar, password no -> but default is fine argument
			- all creation we let dn write
		- normally if the zero value in go is not the zero value in db. do not let go write it.*as always we are taking about INSERTS)

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
