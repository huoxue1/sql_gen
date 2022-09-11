package main

import "github.com/dave/jennifer/jen"

type Driver interface {
	GetDriverName() string
	GetDriverImport() string
	GetDatabaseName(url string) string
	TableInfo(dbName string) (map[string]string, error)
	FieldInfo(dbName, tableName string) ([]*Field, error)
	ConvertType(statement *jen.Statement, columnType string) jen.Code
}

type DB struct {
	Tables []*Table
}

type Table struct {
	Name   string
	Fields []*Field
	Pri    *Field
}

type Field struct {
	fieldName string
	fieldDesc string
	dataType  string
	isAuto    bool
	isNull    string
	columnKey string
	length    int
}
