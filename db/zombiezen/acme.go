package zombiezen

import (
	"fmt"
	"github.com/caasmo/restinpieces/db"
)

func (d *Db) Get() (*db.AcmeCert, error) {
	return nil, fmt.Errorf("DbAcme not implemented for zombiezen SQLite variant")
}
