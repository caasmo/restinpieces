package db

// User represents a user from the database
type User struct {
	ID        string
	Email     string
	Name      string
	Password  string
	Avatar    string
	Created   string
	Updated   string
	Verified  bool
	TokenKey  string
}

type Db interface {
	Close()
	GetById(id int64) int
	Insert(value int64)
	InsertWithPool(value int64)
	GetUserByEmail(email string) (userID string, hashedPassword string, err error)
	CreateUser(email, hashedPassword, name string) (string, error)
}
