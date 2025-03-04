package db

type Db interface {
	Close()
	GetById(id int64) int
	Insert(value int64)
	InsertWithPool(value int64)
}

