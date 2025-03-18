package zombiezen

// TODO not code reviewed. Machine generated R1

import (
	"context"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"runtime"
	"time"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type Db struct {
	pool *sqlitex.Pool
	//rwConn *sqlitex.Conn
	rwCh chan *sqlite.Conn
}

// Verify interface implementation (non-allocating check)
var _ db.Db = (*Db)(nil)

func New(path string) (*Db, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", path)

	p, err := sqlitex.NewPool(initString, sqlitex.PoolOptions{
		Flags:    0, // Use all default flags including WAL
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, err
	}

	conn, err := p.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	// TODO keep track of closing
	//defer db.Put(conn)
	ch := make(chan *sqlite.Conn, 1)
	go func(conn *sqlite.Conn, ch chan *sqlite.Conn) {
		// TODO Use context to select with timeout to cleanly clean up goroutine
		ch <- conn
	}(conn, ch)

	return &Db{pool: p, rwCh: ch}, nil
}

func (d *Db) Close() {
	d.pool.Close()
}

func (d *Db) GetById(id int64) int {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		panic(err) // TODO: Proper error handling
	}
	defer d.pool.Put(conn)

	var value int
	fn := func(stmt *sqlite.Stmt) error {
		//id = int(stmt.GetInt64("id"))
		value = int(stmt.ColumnInt64(0))
		return nil
	}

	if err := sqlitex.Execute(conn, "select value from foo where rowid = ? limit 1", &sqlitex.ExecOptions{
		ResultFunc: fn,
		Args:       []any{id},
	}); err != nil {
		// TODO
		panic(err)
	}

	return value
}

func (d *Db) Insert(value int64) {
	rwConn := <-d.rwCh
	defer func() { d.rwCh <- rwConn }()

	if err := sqlitex.Execute(rwConn, "INSERT INTO foo(id, value) values(1000000,?)", &sqlitex.ExecOptions{
		Args: []any{value},
	}); err != nil {
		// TODO
		panic(err)
	}
}

// GetUserByEmail TODO: Implement for zombiezen SQLite variant
func (d *Db) GetUserByEmail(email string) (*db.User, error) {
	return nil, fmt.Errorf("not implemented for zombiezen SQLite variant")
}

// CreateUser inserts a new user with RFC3339 formatted UTC timestamps
// InsertQueueJob placeholder for zombiezen SQLite implementation
func (d *Db) InsertQueueJob(job queue.QueueJob) error {
	return fmt.Errorf("InsertQueueJob not implemented for zombiezen SQLite variant")
}

func (d *Db) GetUserById(id string) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	var user *db.User
	err = sqlitex.Execute(conn,
		`SELECT id, name, password, verified, externalAuth, avatar, email, emailVisibility, created, updated
		FROM users WHERE id = ? LIMIT 1`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				created, err := db.TimeParse(stmt.GetText("created"))
				if err != nil {
					return fmt.Errorf("error parsing created time: %w", err)
				}

				updated, err := db.TimeParse(stmt.GetText("updated"))
				if err != nil {
					return fmt.Errorf("error parsing updated time: %w", err)
				}

				user = &db.User{
					ID:              stmt.GetText("id"),
					Email:           stmt.GetText("email"),
					Name:            stmt.GetText("name"),
					Password:        stmt.GetText("password"),
					Created:         created,
					Updated:         updated,
					Verified:        stmt.GetInt64("verified") != 0,
					ExternalAuth:    stmt.GetText("externalAuth"),
					EmailVisibility: stmt.GetInt64("emailVisibility") != 0,
				}
				return nil
			},
			Args: []interface{}{id},
		})

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (d *Db) CreateUserWithPassword(user db.User) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	now := time.Now().UTC().Format(time.RFC3339)

	var createdUser db.User
	err = sqlitex.Execute(conn,
		`INSERT INTO users (name, password, verified, externalAuth, avatar, email, emailVisibility, created, updated) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			password = IIF(password = '', excluded.password, password)
		RETURNING id, email, name, password, created, updated, verified`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				created, err := db.TimeParse(stmt.GetText("created"))
				if err != nil {
					return fmt.Errorf("error parsing created time: %w", err)
				}

				updated, err := db.TimeParse(stmt.GetText("updated"))
				if err != nil {
					return fmt.Errorf("error parsing updated time: %w", err)
				}

				createdUser = db.User{
					ID:       stmt.GetText("id"),
					Email:    stmt.GetText("email"),
					Password: stmt.GetText("password"),
					Created:  created,
					Updated:  updated,
					Verified: stmt.GetInt64("verified") != 0,
				}
				return nil
			},
			Args: []interface{}{
				"",         // 1. name
				password,   // 2. password
				false,      // 3. verified
				"",         // 4. externalAuth
				"",         // 5. avatar
				email,      // 6. email
				false,      // 7. emailVisibility
				now,        // 8. created
				now,        // 9. updated
			},
		})

	return &createdUser, err
}

func (d *Db) CreateUser(user db.User) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	// TODO no time object generation in db functions, only formating
	now := time.Now().UTC().Format(time.RFC3339)

	var createdUser db.User
	err = sqlitex.Execute(conn,
		`INSERT INTO users (email, password, name, created, updated)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, email, name, password, created, updated, verified`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {

				// Parse timestamps from database strings
				created, err := db.TimeParse(stmt.GetText("created"))
				if err != nil {
					return fmt.Errorf("error parsing created time: %w", err)
				}

				updated, err := db.TimeParse(stmt.GetText("updated"))
				if err != nil {
					return fmt.Errorf("error parsing updated time: %w", err)
				}

				createdUser = db.User{
					ID:       stmt.GetText("id"),
					Email:    stmt.GetText("email"),
					Name:     stmt.GetText("name"),
					Password: stmt.GetText("password"),
					Created:  created,
					Updated:  updated,
					Verified: stmt.GetInt64("verified") != 0,
				}
				return nil
			},
			Args: []interface{}{
				user.Email,
				user.Password,
				user.Name,
				now,
				now,
			},
		})

	return &createdUser, err
}

func (d *Db) InsertWithPool(value int64) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		panic(err) // TODO: Proper error handling
	}
	defer d.pool.Put(conn)

	if err := sqlitex.Execute(conn, "INSERT INTO foo(id, value) values(1000000,?)", &sqlitex.ExecOptions{
		Args: []any{value},
	}); err != nil {
		// TODO
		panic(err)
	}
}
