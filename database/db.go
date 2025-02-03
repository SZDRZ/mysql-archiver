package database

import (
	"database/sql"
	"mysql-archiver/config"
	"time"

	"github.com/go-sql-driver/mysql"
)

func InitDataSource(cfg *config.Config) (src, dst *sql.DB, err error) {

	src, err = sql.Open("mysql", (&mysql.Config{
		Net:                  "tcp",
		Addr:                 cfg.Global.Datasource.Src.Addr,
		User:                 cfg.Global.Datasource.Src.User,
		Passwd:               cfg.Global.Datasource.Src.Pass,
		DBName:               cfg.Global.Datasource.Src.Dbname,
		AllowNativePasswords: true,
		ParseTime:            true,
		Timeout:              time.Second * 3,
	}).FormatDSN())

	if err != nil {
		return
	}

	err = src.Ping()
	if err != nil {
		src.Close()
		src = nil
		return
	}

	dst, err = sql.Open("mysql", (&mysql.Config{
		Net:                  "tcp",
		Addr:                 cfg.Global.Datasource.Dst.Addr,
		User:                 cfg.Global.Datasource.Dst.User,
		Passwd:               cfg.Global.Datasource.Dst.Pass,
		DBName:               cfg.Global.Datasource.Dst.Dbname,
		AllowNativePasswords: true,
		Timeout:              time.Second * 3,
	}).FormatDSN())

	if err != nil {
		return
	}

	err = dst.Ping()
	if err != nil {
		dst.Close()
		dst = nil
		return
	}

	return

}
