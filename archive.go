package main

import (
	"bytes"
	"database/sql"
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

	if tr.Previous == nil {
		sql := fmt.Sprintf("create temporary table tmp_%s as select %s from %s where %s limit %d", tr.Table, tr.Pk, tr.Table, tr.Where, tr.Batch_size)
		sqlResult, err := dsSrc.Exec(sql)
		if err != nil {
			log.Println("主表", tr.Table, "查询主键列执行失败:", err)
			return err
		}

		rowsAffect, _ := sqlResult.RowsAffected()
		log.Println("主表", tr.Table, "查询行数:", rowsAffect)
	} else {
		sql := fmt.Sprintf("create temporary table tmp_%s as select %s from %s where %s in (table tmp_%s)", tr.Table, tr.Pk, tr.Table, tr.Key, tr.Previous.Table)
		sqlResult, err := dsSrc.Exec(sql)
		if err != nil {
			log.Println("子表", tr.Table, "查询主键列执行失败:", err)
			return err
		}

		rowsAffect, _ := sqlResult.RowsAffected()
		log.Println("子表", tr.Table, "查询行数:", rowsAffect)
	}

	defer dsSrc.Exec("drop temporary table tmp_" + tr.Table)

	if len(tr.Deps) > 0 {
		for _, rule := range tr.Deps {
			rule.Previous = tr
			if err := DepsHandler(&rule, dsSrc, dsSrc); err != nil {
				return err
			}
		}
	}

	sql := fmt.Sprintf("select * from %s where %s in (table tmp_%s)", tr.Table, tr.Pk, tr.Table)
	queryResult, err := dsSrc.Query(sql)
	if err != nil {
		log.Println("表", tr.Table, "数据查询失败:", err)
		return err
	}

	buffer := QueryToTSV(queryResult)
	if buffer == nil {
		log.Println("表", tr.Table, "数据转TSV失败!")
		return err
	}

	mysql.RegisterReaderHandler("data", func() io.Reader {
		return buffer
	})

	loadResult, err := dsDst.Exec(`load data local infile 'Reader::data' into table ` + tr.Table)
	if err != nil {
		log.Println("表", tr.Table, "数据导入失败:", err)
		return err
	}

	loadedRows, _ := loadResult.RowsAffected()
	log.Println("表", tr.Table, "已成功导入", loadedRows, "行!")

	sql = fmt.Sprintf(`delete from %s where %s in (table tmp_%s)`, tr.Table, tr.Pk, tr.Table)
	sqlResult, err := dsSrc.Exec(sql)
	if err != nil {
		log.Println("表", tr.Table, "删除原数据失败!", err)
		return err
	}

	deletedRows, _ := sqlResult.RowsAffected()
	log.Println("表", tr.Table, "原始数据已删除", deletedRows, "行!")

	return nil

}

func Archive(cfg *config.Config, srcSqlHandler, dstSqlHandler *sql.DB) {

	srcSqlHandler.SetMaxOpenConns(1)
	dstSqlHandler.SetMaxOpenConns(1)

	srcSqlHandler.Exec("SET @@SESSION.TRANSACTION_ISOLATION='" + cfg.Datasource.Transaction_isolation + "'")

	for _, rule := range cfg.Rules {
		if err := DepsHandler(&rule, srcSqlHandler, dstSqlHandler); err != nil {
			continue
		}
		time.Sleep(cfg.Global.Sleep)
	}

}
