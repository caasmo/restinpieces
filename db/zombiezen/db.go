package zombiezen

import (
	"context"
	"fmt"
	"github.com/caasmo/restinpieces/db"
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
func (d *Db) CreateUser(user db.User) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	now := time.Now().UTC().Format(time.RFC3339)
	
	var createdUser db.User
	err = sqlitex.Execute(conn,
		`INSERT INTO users (email, password, name, created, updated, tokenKey)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id, email, name, password, created, updated, verified, tokenKey`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				createdUser = db.User{
					ID:       stmt.GetText("id"),
					Email:    stmt.GetText("email"),
					Name:     stmt.GetText("name"),
					Password: stmt.GetText("password"),
					Created:  stmt.GetText("created"),
					Updated:  stmt.GetText("updated"),
					Verified: stmt.GetInt64("verified") != 0,
					TokenKey: stmt.GetText("tokenKey"),
				}
				return nil
			},
			Args: []interface{}{
				user.Email,
				user.Password,
				user.Name,
				now,
				now,
				user.TokenKey,
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
