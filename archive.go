package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mysql-archiver/config"
	"strconv"
	"strings"
	"text/template"
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

func sqlfmt(sql_template string, data interface{}) string {
	tpl := template.New("default")
	_, err := tpl.Parse(sql_template)
	if err != nil {
		log.Println("[ERROR] sql template parse failed:", err)
		return ""
	}
	sb := &strings.Builder{}
	tpl.Execute(sb, data)
	s := sb.String()

	log.Println("[DEBUG] QUERY:", s)

	return s
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

func DepsHandler(global_cfg *config.Config, tr *config.TableRule, dsSrc, dsDst *sql.DB) error {

	// sql := fmt.Sprintf("create temporary table tmp_%s as select %s from %s limit 0", tr.Table, tr.Pk, tr.Table)
	// log.Println("[DEBUG] QUERY:", sql)

	_, err := dsSrc.Exec(sqlfmt(
		"create temporary table tmp_{{.Table}} as select {{.Pk}} from {{.Table}} limit 0",
		map[string]string{
			"Table": tr.Table,
			"Pk":    tr.Pk,
		},
	))
	if err != nil {
		return err
	}

	defer func() {
		dsSrc.Exec(sqlfmt(
			`drop temporary table if exists tmp_{{.Table}}`,
			map[string]string{
				"Table": tr.Table,
			},
		))
	}()

	for {

		if tr.Previous == nil {

			sql := sqlfmt(
				"insert into tmp_{{.Table}} select {{.Pk}} from {{.Table}} where {{.Where}} limit {{.BatchSize}}",
				map[string]string{
					"Table":     tr.Table,
					"Pk":        tr.Pk,
					"Where":     tr.Where,
					"BatchSize": strconv.Itoa(global_cfg.Global.Batch_size),
				},
			)
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

			sql := sqlfmt(
				"insert into tmp_{{.Table}} select {{.Pk}} from {{.Table}} where {{.Key}} in (table tmp_{{.PreviousTable}})",
				map[string]string{
					"Table":         tr.Table,
					"Pk":            tr.Pk,
					"Key":           tr.Key,
					"PreviousTable": tr.Previous.Table,
				},
			)
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
				if err := DepsHandler(global_cfg, &rule, dsSrc, dsDst); err != nil {
					return err
				}
			}
		}

		/* 查匹配的全列数据 */
		sql := sqlfmt(
			"select * from {{.Table}} where {{.Pk}} in (table tmp_{{.Table}})",
			map[string]string{
				"Table": tr.Table,
				"Pk":    tr.Pk,
			},
		)
		queryResult, err := dsSrc.Query(sql)
		if err != nil {
			log.Println("table", tr.Table, "fetch data error:", err)
			return err
		}

		/* 转换为TSV格式 */
		buffer := QueryToTSV(queryResult)
		if buffer == nil {
			log.Println("table", tr.Table, "transform to tsv error!")
			return errors.New("transform tsv error")
		}

		log.Println("buffer size:", buffer.Len())

		/* 导入数据 */
		mysql.RegisterReaderHandler("data", func() io.Reader {
			return buffer
		})

		sql = sqlfmt(`load data local infile 'Reader::data' into table {{.Table}}`, map[string]string{
			"Table": tr.Table,
		})
		loadResult, err := dsDst.Exec(sql)
		if err != nil {
			log.Println("table", tr.Table, "load data error:", err)
			return err
		}

		loadedRows, _ := loadResult.RowsAffected()
		if loadedRows == 0 {
			log.Println("table", tr.Table, "load failed! number of rows equal zero.")

			r, _ := dsDst.Query("show warnings")
			for r.Next() {
				var c1, c2, c3 string
				r.Scan(&c1, &c2, &c3)
				fmt.Println(c1, c2, c3)
			}
			r.Close()
			return errors.New("LOAD_ZERO_ROWS")
		}
		log.Println("table", tr.Table, "success loaded", loadedRows, "rows")

		/* 删除源数据 */
		sql = sqlfmt(
			`delete from {{.Table}} where {{.Pk}} in (table tmp_{{.Table}})`,
			map[string]string{
				"Table": tr.Table,
				"Pk":    tr.Pk,
			},
		)
		sqlResult, err := dsSrc.Exec(sql)
		if err != nil {
			log.Println("table", tr.Table, "delete origin data error:", err)
			return err
		}

		deletedRows, _ := sqlResult.RowsAffected()
		log.Println("table", tr.Table, "has been deleted", deletedRows, "rows")

		/* 清空临时表 */
		sql = sqlfmt(
			`truncate table tmp_{{.Table}}`,
			map[string]string{
				"Table": tr.Table,
			},
		)
		dsSrc.Exec(sql)

		time.Sleep(global_cfg.Global.Sleep)
	}

	return nil

}

func Archive(cfg *config.Config, srcSqlHandler, dstSqlHandler *sql.DB) {

	srcSqlHandler.SetMaxOpenConns(1)
	dstSqlHandler.SetMaxOpenConns(1)

	srcSqlHandler.Exec("SET @@SESSION.TRANSACTION_ISOLATION='" + cfg.Global.Datasource.Transaction_isolation + "'")

	for _, rule := range cfg.Rules {
		if err := DepsHandler(cfg, &rule, srcSqlHandler, dstSqlHandler); err != nil {
			log.Println("[ERROR] table ", rule.Table, "archive failed! skipped...")
			continue
		}
	}

}
