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
		Addr:                 cfg.Datasource.Src.Addr,
		User:                 cfg.Datasource.Src.User,
		Passwd:               cfg.Datasource.Src.Pass,
		DBName:               cfg.Datasource.Src.Dbname,
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
		Addr:                 cfg.Datasource.Dst.Addr,
		User:                 cfg.Datasource.Dst.User,
		Passwd:               cfg.Datasource.Dst.Pass,
		DBName:               cfg.Datasource.Dst.Dbname,
		AllowNativePasswords: true,
		AllowAllFiles:        true,
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
