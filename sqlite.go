package main

import (
	"regexp"
	"strings"

	"github.com/dave/jennifer/jen"
	log "github.com/sirupsen/logrus"
)

type sqlite struct {
}

func (s *sqlite) GetDriverName() string {
	return "sqlite"
}

func (s *sqlite) GetDriverImport() string {
	return "modernc.org/sqlite"
}

func (s *sqlite) GetDatabaseName(url string) string {
	return ""
}

func (s *sqlite) TableInfo(dbName string) (map[string]string, error) {
	rows, err := db.Query("select tbl_name from sqlite_master where type='table' and name != 'sqlite_sequence'")
	if err != nil {
		return nil, err
	}
	results := map[string]string{}
	for rows.Next() {
		var tbl_name string
		err := rows.Scan(&tbl_name)
		if err != nil {
			return nil, err
		}
		results[tbl_name] = ""
	}
	return results, err
}

func (s *sqlite) FieldInfo(dbName, tableName string) ([]*Field, error) {
	var sql string
	err := db.QueryRow(`select sql from sqlite_master where tbl_name=?`, tableName).Scan(&sql)
	if err != nil {
		return nil, err
	}
	compile, err := regexp.Compile(`\((.*?)\)`)
	if err != nil {
		return nil, err
	}
	allString := compile.FindAllStringSubmatch(strings.ReplaceAll(strings.ToLower(sql), "\r\n", ""), -1)
	data := allString[0][1]
	log.Debugln(data)
	var fields []*Field
	for _, s2 := range strings.Split(data, ",") {
		s2 = strings.TrimSpace(s2)
		keys := strings.Split(s2, " ")
		trimSpace := func(datas []string) []string {
			var newData []string
			for _, s3 := range datas {
				if s3 != "" {
					newData = append(newData, s3)
				}
			}
			return newData
		}
		keys = trimSpace(keys)
		field := new(Field)
		field.fieldName = keys[0]
		field.dataType = keys[1]
		hasPri := false
		hasKey := false
		for _, key := range keys {
			if key == "primary" {
				hasPri = true
			}
			if key == "key" {
				hasKey = true
			}
			if key == "autoincrement" {
				field.isAuto = true
			}
		}
		if hasKey && hasPri {
			field.columnKey = "PRI"
		}
		fields = append(fields, field)
	}
	return fields, err
}

func (s *sqlite) ConvertType(statement *jen.Statement, columnType string) jen.Code {
	switch columnType {
	case "varchar", "char", "tinytext", "longtext", "mediumtext", "text":
		return statement.String()
	case "tinyint", "int", "smallint", "mediumint", "integer", "bigint":
		return statement.Int()
	case "float", "double", "decimal", "real":
		return statement.Float64()
	case "tinyblob", "blob", "mediumblob", "longblob":
		return statement.Op("[]").Byte()
	case "date", "time", "year", "datetime", "timestamp":
		return statement.Qual("time", "Time")
	default:
		return statement.String()
	}
}
