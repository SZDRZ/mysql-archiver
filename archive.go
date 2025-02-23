package main

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"mysql-archiver/config"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	FIELD_SPLIT_CHAR = ','
	LINE_SPLIT_CHAR  = '\n'
	// ESCAPE_CHAR            = '\\'
	// MYSQL_DATETIME6_FORMAT = time.DateTime + ".000000"
)

// var (
// 	ESCAPE_CHARS = map[rune]struct{}{
// 		LINE_SPLIT_CHAR:  {}, // \n
// 		FIELD_SPLIT_CHAR: {}, // \t
// 		0x0D:             {}, // \r
// 		0x08:             {}, // \b
// 		0x00:             {}, // \0
// 		0x1A:             {}, // \Z
// 		ESCAPE_CHAR:      {}, // '\'
// 	}
// )

// func escape(in *string) (out *string) {

// 	t := strings.Builder{}

// 	for _, c := range *in {
// 		if _, exists := ESCAPE_CHARS[c]; exists {
// 			t.WriteByte(ESCAPE_CHAR)
// 		}
// 		t.WriteRune(c)
// 	}

// 	s := t.String()

// 	return &s
// }

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

/*
input: []byte(xxx)
output: x'A1B2C3'
*/
func encodeToHex(b []byte) string {
	s := strings.Builder{}
	s.WriteString("x'")
	s.WriteString(hex.EncodeToString(b))
	s.WriteString("'")
	return s.String()
}

/*
output: (...),(...),(...)
*/
func QueryToInsertInto(queryResult *sql.Rows) []byte {

	defer queryResult.Close()

	columnTypes, _ := queryResult.ColumnTypes()
	resultScanList := make([]interface{}, len(columnTypes))
	for i, columnType := range columnTypes {
		switch typeName := columnType.DatabaseTypeName(); typeName {
		case
			"TINYINT", "SMALLINT", "MEDIUMINT", "INT", "BIGINT",
			"UNSIGNED TINYINT", "UNSIGNED SMALLINT", "UNSIGNED MEDIUMINT", "UNSIGNED INT", "UNSIGNED BIGINT",
			"DECIMAL", "DOUBLE", "FLOAT":

			resultScanList[i] = new(sql.NullString)
		case
			"CHAR", "VARCHAR",
			"TINYTEXT", "TEXT", "MEDIUMTEXT", "LONGTEXT",
			"TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB",
			"JSON", "BIT",
			"TIME", "DATE", "YEAR", "DATETIME", "TIMESTAMP":

			resultScanList[i] = new(sql.RawBytes)
		default:
			log.Printf("column:%s type:%s is not support yet!\n", columnType.Name(), typeName)
			return nil
		}
	}

	tmpBuffer := bytes.NewBuffer(nil)
	// tmpBuffer.WriteString("INSERT INTO `" + table + "` VALUES ")

	for queryResult.Next() {
		err := queryResult.Scan(resultScanList...)
		if err != nil {
			log.Println("data scan error:", err)
			return nil
		}

		tmpBuffer.WriteByte('(')

		for i, colPointer := range resultScanList {
			switch val := colPointer.(type) {

			case *sql.NullString:
				if val.Valid {

					_, err := tmpBuffer.WriteString(val.String)
					if err != nil {
						log.Println("write buffer error:", err)
						return nil
					}

				} else {
					_, err := tmpBuffer.WriteString("NULL")
					if err != nil {
						log.Println("write buffer error:", err)
						return nil
					}
				}
			case *sql.RawBytes:
				if len(*val) == 0 {
					tmpBuffer.WriteString("NULL")
				} else {
					tmpBuffer.WriteString(encodeToHex(*val))
				}

			}

			// 最后一列不需要加 列分隔符
			if i < len(resultScanList)-1 {
				tmpBuffer.WriteByte(FIELD_SPLIT_CHAR)
			}

		}

		tmpBuffer.WriteString("),")

	}

	return tmpBuffer.Bytes()[:tmpBuffer.Len()-1]

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

		if tr.Previous == nil { /* 父节点处理 */

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

		} else { /* 子节点处理 */

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

		/* 拼接成INSERT语句导入 */
		insertbuf := QueryToInsertInto(queryResult)
		if insertbuf == nil {
			log.Println("table", tr.Table, "transform to insert sql error!")
			return errors.New("transform to insert sql error")
		}
		sqlbuf := bytes.NewBuffer(nil)
		sqlbuf.WriteString("INSERT INTO `" + tr.Table + "` VALUES ")
		sqlbuf.Write(insertbuf)

		log.Println("insert sql buf size:", sqlbuf.Len())

		loadResult, err := dsDst.Exec(sqlbuf.String())
		if err != nil {
			log.Println("table", tr.Table, "load data error:", err)
			return err
		}
		loadRows, _ := loadResult.RowsAffected()
		log.Println("table", tr.Table, "success loaded", loadRows, "rows")

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
