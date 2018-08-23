package missinggo

import (
	"database/sql"
	"time"
)

type SqliteTime time.Time

var _ sql.Scanner = (*SqliteTime)(nil)

func (me *SqliteTime) Scan(src interface{}) error {
	var tt time.Time
	tt, err := time.Parse("2006-01-02 15:04:05", string(src.([]byte)))
	*me = SqliteTime(tt)
	return err
}
