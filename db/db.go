package db

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
    "runtime"
    "fmt"
)

// maybe interface? no: use 
type Db struct {
    *sqlitex.Pool
}

//
func New(path string) (*Db, error) {
	poolSize := runtime.NumCPU()
    initString := fmt.Sprintf("file:%s", path)

	db, err := sqlitex.Open(initString, 0, poolSize)
	if err != nil {
        return &Db{}, err
	}

    return &Db{db}, nil
}

func (db *Db) Close() {
    db.Close()
}

func (db *Db) GetById(id int) int {
    conn := db.Get(nil)
    defer db.Put(conn)

    var value int
    fn := func(stmt *sqlite.Stmt) error {
        //id = int(stmt.GetInt64("id"))
        value = int(stmt.GetInt64("value"))
        return nil
    }

    if err := sqlitex.Exec(conn, "select value from foo where rowid = ? limit 1", fn, id); err != nil {
        // TODO
        panic(err)
    }

    return value
}



