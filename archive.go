package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mysql-archiver/config"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	FIELD_SPLIT_CHAR       = '\t'
	LINE_SPLIT_CHAR        = '\n'
	ESCAPE_CHAR            = '\\'
	MYSQL_DATETIME6_FORMAT = time.DateTime + ".000000"
)

var (
	ESCAPE_CHARS = map[rune]struct{}{
		LINE_SPLIT_CHAR:  {}, // \n
		FIELD_SPLIT_CHAR: {}, // \t
		0x0D:             {}, // \r
		0x08:             {}, // \b
		0x00:             {}, // \0
		0x1A:             {}, // \Z
		ESCAPE_CHAR:      {}, // '\'
	}
)

func escape(in *string) (out *string) {

	t := strings.Builder{}

	for _, c := range *in {
		if _, exists := ESCAPE_CHARS[c]; exists {
			t.WriteByte(ESCAPE_CHAR)
		}
		t.WriteRune(c)
	}

	s := t.String()

	return &s
}

func QueryToTSV(queryResult *sql.Rows) *bytes.Buffer {

	defer queryResult.Close()

	columnTypes, _ := queryResult.ColumnTypes()
	resultScanList := make([]interface{}, len(columnTypes))
	for i, columnType := range columnTypes {
		switch typeName := columnType.DatabaseTypeName(); typeName {
		case
			"TINYINT", "UNSIGNED TINYINT", "SMALLINT", "UNSIGNED SMALLINT", "MEDIUMINT", "UNSIGNED MEDIUMINT", "INT", "UNSIGNED INT", "BIGINT", "UNSIGNED BIGINT",
			"DECIMAL", "DOUBLE", "FLOAT",
			"CHAR", "VARCHAR", "TINYTEXT", "TEXT", "MEDIUMTEXT", "LONGTEXT",
			"JSON", "BIT":
			canbeNull, _ := columnType.Nullable()
			if canbeNull {
				resultScanList[i] = new(sql.NullString)
			} else {
				resultScanList[i] = new(string)
			}
		case "DATETIME", "DATE", "TIMESTAMP":
			canbeNull, _ := columnType.Nullable()
			if canbeNull {
				resultScanList[i] = new(sql.NullTime)
			} else {
				resultScanList[i] = new(time.Time)
			}
		default:
			log.Printf("column:%s type:%s is not support yet!\n", columnType.Name(), typeName)
			return nil
		}
	}

	dataBuffer := bytes.NewBuffer(nil)

	for queryResult.Next() {
		err := queryResult.Scan(resultScanList...)
		if err != nil {
			log.Println("data scan error:", err)
			return nil
		}

		for i, colPointer := range resultScanList {
			switch val := colPointer.(type) {
			case *string:
				_, err := dataBuffer.WriteString(*escape(val))
				if err != nil {
					log.Println("write buffer error:", err)
					return nil
				}
			case *sql.NullString:
				if val.Valid {
					_, err := dataBuffer.WriteString(*escape(&val.String))
					if err != nil {
						log.Println("write buffer error:", err)
						return nil
					}
				} else {
					_, err := dataBuffer.WriteString("\\N")
					if err != nil {
						log.Println("write buffer error:", err)
						return nil
					}
				}
			case *time.Time:
				_, err := dataBuffer.WriteString(val.Format(MYSQL_DATETIME6_FORMAT))
				if err != nil {
					log.Println("write buffer error:", err)
					return nil
				}
			case *sql.NullTime:
				if val.Valid {
					_, err := dataBuffer.WriteString(val.Time.Format(MYSQL_DATETIME6_FORMAT))
					if err != nil {
						log.Println("write buffer error:", err)
						return nil
					}
				} else {
					_, err := dataBuffer.WriteString("\\N")
					if err != nil {
						log.Println("write buffer error:", err)
						return nil
					}
				}
			}

			// 最后一列不需要加 列分隔符
			if i < len(resultScanList)-1 {
				dataBuffer.WriteByte(FIELD_SPLIT_CHAR)
			}
		}

		dataBuffer.WriteByte(LINE_SPLIT_CHAR)
	}

	return dataBuffer

}

func DepsHandler(tr *config.TableRule, dsSrc, dsDst *sql.DB) error {

	sql := fmt.Sprintf("create temporary table tmp_%s as select %s from %s limit 0", tr.Table, tr.Pk, tr.Table)
	log.Println("[DEBUG] QUERY:", sql)
	_, err := dsSrc.Exec(sql)
	if err != nil {
		return err
	}

	defer func() {
		sql := `drop temporary table if exists tmp_` + tr.Table
		log.Println("[DEBUG] QUERY:", sql)
		dsSrc.Exec(sql)
	}()

	for {

		if tr.Previous == nil {
			sql := fmt.Sprintf("insert into tmp_%s select %s from %s where %s limit %d", tr.Table, tr.Pk, tr.Table, tr.Where, tr.Batch_size)
			log.Println("[DEBUG] QUERY:", sql)
			sqlResult, err := dsSrc.Exec(sql)
			if err != nil {
				log.Println("[ERROR] select primary table", tr.Table, "pk records error:", err)
				return err
			}

			rowsAffect, _ := sqlResult.RowsAffected()
			log.Println("select", rowsAffect, "rows in primary table")
			if rowsAffect == 0 {
				break
			}
		} else {
			sql := fmt.Sprintf("insert into tmp_%s select %s from %s where %s in (table tmp_%s)", tr.Table, tr.Pk, tr.Table, tr.Key, tr.Previous.Table)
			log.Println("[DEBUG] QUERY:", sql)
			sqlResult, err := dsSrc.Exec(sql)
			if err != nil {
				log.Println("[ERROR] select sub table", tr.Table, "pk records error:", err)
				return err
			}

			rowsAffect, _ := sqlResult.RowsAffected()
			log.Println("select", rowsAffect, "rows in sub table")
			if rowsAffect == 0 {
				break
			}
		}

		if len(tr.Deps) > 0 {
			for _, rule := range tr.Deps {
				rule.Previous = tr
				if err := DepsHandler(&rule, dsSrc, dsSrc); err != nil {
					return err
				}
			}
		}

		sql := fmt.Sprintf("select * from %s where %s in (table tmp_%s)", tr.Table, tr.Pk, tr.Table)
		log.Println("[DEBUG] QUERY:", sql)
		queryResult, err := dsSrc.Query(sql)
		if err != nil {
			log.Println("table", tr.Table, "fetch data error:", err)
			return err
		}

		buffer := QueryToTSV(queryResult)
		if buffer == nil {
			log.Println("table", tr.Table, "transform to tsv error!")
			return errors.New("transform tsv error")
		}

		log.Println("buffer size:", buffer.Len())

		mysql.RegisterReaderHandler("data", func() io.Reader {
			return buffer
		})

		loadResult, err := dsDst.Exec(`load data local infile 'Reader::data' into table ` + tr.Table)
		if err != nil {
			log.Println("table", tr.Table, "load data error:", err)
			return err
		}

		loadedRows, _ := loadResult.RowsAffected()
		log.Println("table", tr.Table, "success loaded", loadedRows, "rows")

		sql = fmt.Sprintf(`delete from %s where %s in (table tmp_%s)`, tr.Table, tr.Pk, tr.Table)
		log.Println("[DEBUG] QUERY:", sql)
		sqlResult, err := dsSrc.Exec(sql)
		if err != nil {
			log.Println("table", tr.Table, "delete origin data error:", err)
			return err
		}

		deletedRows, _ := sqlResult.RowsAffected()
		log.Println("table", tr.Table, "has been deleted", deletedRows, "rows")

		sql = `truncate table tmp_` + tr.Table
		log.Println("[DEBUG] QUERY:", sql)
		dsSrc.Exec(sql)

		time.Sleep(time.Second)
	}

	return nil

}

func Archive(cfg *config.Config, srcSqlHandler, dstSqlHandler *sql.DB) {

	srcSqlHandler.SetMaxOpenConns(1)
	dstSqlHandler.SetMaxOpenConns(1)

	srcSqlHandler.Exec("SET @@SESSION.TRANSACTION_ISOLATION='" + cfg.Global.Datasource.Transaction_isolation + "'")

	for _, rule := range cfg.Rules {
		if err := DepsHandler(&rule, srcSqlHandler, dstSqlHandler); err != nil {
			log.Println("[ERROR] table ", rule.Table, "archive failed! skipped...")
			continue
		}
	}

}
