package main

import (
	"strings"

	"github.com/dave/jennifer/jen"
	log "github.com/sirupsen/logrus"
)

type mysql struct {
}

func (m *mysql) GetDriverImport() string {
	return "github.com/go-sql-driver/mysql"
}

func (m *mysql) GetDriverName() string {
	return "mysql"
}

func (m *mysql) GetDatabaseName(url string) string {
	return strings.Split(url, "/")[1]
}

func (m *mysql) ConvertType(statement *jen.Statement, columnType string) jen.Code {
	switch columnType {
	case "varchar", "char", "tinytext", "longtext", "mediumtext":
		return statement.String()
	case "tinyint", "int", "smallint", "mediumint", "integer", "bigint":
		return statement.Int()
	case "float", "double", "decimal":
		return statement.Float64()
	case "tinyblob", "blob", "mediumblob", "longblob":
		return statement.Op("[]").Byte()
	case "date", "time", "year", "datetime", "timestamp":
		return statement.Qual("time", "Time")
	default:
		return statement.String()
	}
}

func (m *mysql) TableInfo(dbName string) (map[string]string, error) {
	sqlStr := `select TABLE_NAME tableName,TABLE_COMMENT tableDesc 
			from information_schema.TABLES where lower(TABLE_SCHEMA) = ?`

	var result = make(map[string]string)

	rows, err := db.Query(sqlStr, dbName)
	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	for rows.Next() {
		var tableName, tableDesc string
		err = rows.Scan(&tableName, &tableDesc)
		if err != nil {
			return nil, err
		}

		if len(tableDesc) == 0 {
			tableDesc = tableName
		}
		result[tableName] = tableDesc
	}
	return result, err
}

func (m *mysql) FieldInfo(dbName, tableName string) ([]*Field, error) {
	sqlStr := `SELECT COLUMN_NAME fName,column_comment fDesc,DATA_TYPE dataType,
						IS_NULLABLE isNull,COLUMN_KEY columnKey,IFNULL(CHARACTER_MAXIMUM_LENGTH,0) sLength, EXTRA extra
			FROM information_schema.columns 
			WHERE table_schema = ? AND table_name = ?`

	var result []*Field

	rows, err := db.Query(sqlStr, dbName, tableName)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		f := new(Field)
		var extra string
		err = rows.Scan(&f.fieldName, &f.fieldDesc, &f.dataType, &f.isNull, &f.columnKey, &f.length, &extra)
		if err != nil {
			return nil, err
		}
		if extra == "auto_increment" {
			f.isAuto = true
		}
		result = append(result, f)
	}
	return result, err
}
