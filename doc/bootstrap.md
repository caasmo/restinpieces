The End-User Workflow (Bootstrapping a restinpieces App)

    This is how a developer would use the framework:

    1. Prerequisites: The developer has Go and the age tool installed. They generate their master encryption key:


    1     age-keygen -o age.key


    2. Step 1: Create the Application Instance (`ripc`)
    The developer uses the CLI to create the core application database and the initial, default configuration.


    1     ripc -age-key age.key -dbpath ./myapp.db app create

    * Result: A myapp.db file is created containing the application schema (users, app_config, etc.) and one encrypted
    configuration entry.

   3. Step 2: Customize Configuration (`ripc` + Editor)
      The developer will almost certainly want to customize the configuration (e.g., set JWT secrets, SMTP settings, and
importantly, the log database path).


   1     # Dump the default config to a file
   2     ripc -age-key age.key -dbpath ./myapp.db config dump > config.toml
   3
   4     # Edit config.toml with a text editor
   5     # For example, they add or uncomment:
   6     # [log.batch]
   7     # db_path = "/var/log/restinpieces/prod.log.db"



   4. Step 3: Save the Custom Configuration (`ripc`)
      The developer saves their customized config.toml back into the secure store as the new "latest" version.

   1     ripc -age-key age.key -dbpath ./myapp.db config save config.toml



   5. Step 4: Initialize the Logger Database (`ripc`) 
      This is the ideal, explicit point to initialize the logger. The main application is configured, so we know exactly
where the log database should be.

  2     ripc -age-key age.key -dbpath ./myapp.db log init

       * Action: This command reads the latest config from myapp.db, finds the log.batch.db_path, connects to it (creatin
g the file), and applies the logs.sql schema.
       * Benefit: It's a clean, one-time setup action that prepares the environment for the application.


   6. Step 5: Write the Application Code (Go)
      The developer writes their main.go (like main.go.orig), which calls restinpieces.New() and srv.Run(). This is the s
table, long-term entry point to their server.

   7. Step 6: Run the Application (Go)
      The developer compiles and runs their server.


   1     go run ./cmd/myapp/main.go -dbpath ./myapp.db -age-key ./age.key

       * When restinpieces.New() is called, the setupDefaultLogger function will find the log database path in the config
 and connect to it. Since Step 4 was
         completed, the file and its schema are already present and waiting. The application starts cleanly with no schem
a-check logic.

