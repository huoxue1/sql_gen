package main

import (
	"database/sql"
	"flag"
	"fmt"
	"strings"
	"time"

	nested "github.com/Lyrics-you/sail-logrus-formatter/sailor"
	"github.com/dave/jennifer/jen"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

var (
	sqlType string
	dbUrl   string
	output  string
)

var db *sql.DB

func init() {
	log.SetFormatter(&nested.Formatter{
		FieldsOrder:           nil,
		TimeStampFormat:       "2006-01-02 15:04:05",
		CharStampFormat:       "",
		HideKeys:              false,
		Position:              false,
		Colors:                true,
		FieldsColors:          true,
		FieldsSpace:           true,
		ShowFullLevel:         false,
		LowerCaseLevel:        true,
		TrimMessages:          true,
		CallerFirst:           false,
		CustomCallerFormatter: nil,
	})
}

func init() {

	flag.StringVar(&sqlType, "db_name", "mysql", "the db name!")
	flag.StringVar(&dbUrl, "db_url", "root:123@/test1", "mysql: username:password@tcp(host:port)/database_name\nsqlite: file:path")
	flag.StringVar(&output, "output", "models", "the output dir")
	flag.Parse()
	db1, err := sql.Open(sqlType, dbUrl)
	if err != nil {
		log.Fatalln("连接数据库失败" + err.Error())
	}
	db = db1
}

var (
	drivers = map[string]Driver{
		"mysql":  &mysql{},
		"sqlite": &sqlite{},
	}
)

func main() {
	log.Infoln("开始连接数据库" + sqlType)
	d := drivers[sqlType]
	dbName := d.GetDatabaseName(dbUrl)
	d2 := DB{}
	tableInfo, err := d.TableInfo(dbName)
	if err != nil {
		log.Errorln("获取数据库表错误" + err.Error())
		return
	}
	for tableName := range tableInfo {
		table := Table{}
		table.Name = tableName
		fields, err := d.FieldInfo(dbName, tableName)
		if err != nil {
			log.Errorln("获取字段出现错误" + err.Error())
			return
		}
		table.Fields = fields
		for _, field := range fields {
			if field.columnKey == "PRI" {
				table.Pri = field
			}
		}
		d2.Tables = append(d2.Tables, &table)

	}
	_ = generateDbFile(d)
	for _, table := range d2.Tables {
		generateTable(d, table)
	}
	log.Infoln("数据生成成功")

}

func generateDbFile(driver Driver) error {
	f := jen.NewFile("model")
	f.HeaderComment("generate by sql_gen")
	f.HeaderComment("generate time in " + time.Now().Format("2006-01-02 15:04:05"))
	f.ImportAlias("github.com/sirupsen/logrus", "log")
	f.Anon(driver.GetDriverImport())
	f.Var().Id("db").Op("*").Qual("database/sql", "DB").Empty()

	f.Func().Id("init").Params().Block(
		jen.Var().Id("err").Error(),
		jen.ListFunc(func(group *jen.Group) {
			group.Id("db")
			group.Id("err")
		}).Op("=").Qual("database/sql", "Open").Call(jen.Lit(sqlType), jen.Lit(dbUrl)),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Qual("github.com/sirupsen/logrus", "Fatalln").Call(jen.Lit("打开数据库出现错误"), jen.Id("err.Error").Call()),
		),
	)

	f.Func().Id("ping").Params().Block(
		jen.Id("err").Op(":=").Id("db.Ping").Call(),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Qual("github.com/sirupsen/logrus", "Errorln").Call(jen.Lit("数据库ping出现错误"), jen.Id("err.Error").Call()),
		),
	)
	log.Debugf("%#v\n", f)
	err := f.Save(fmt.Sprintf("./%s/db.go", output))
	if err != nil {
		log.Errorln("保存文件失败" + err.Error())
		return err
	}
	return nil

}

// generateTable
/* @Description: 根据table生成一个对应文件
*  @param driver
*  @param table
 */
func generateTable(driver Driver, table *Table) {
	f := jen.NewFile("model")
	f.HeaderComment("generate by sql_gen")
	f.HeaderComment("generate time in" + time.Now().Format("2006-01-02 15:04:05"))
	f.ImportAlias("github.com/sirupsen/logrus", "log")

	var fields []jen.Code
	for _, field := range table.Fields {
		fields = append(fields, jen.Comment(fmt.Sprintf("column_name:%v, column_type:%v", field.fieldName, field.dataType)))
		fields = append(fields, driver.ConvertType(jen.Id(Case2Camel(field.fieldName)), field.dataType).(*jen.Statement).Tag(map[string]string{
			"json": field.fieldName,
			"db":   field.fieldName,
			"yaml": field.fieldName,
		}))
	}
	jen.Comment("" + Case2Camel(table.Name))
	f.Type().Id(Case2Camel(table.Name)).Struct(fields...)

	generateAddFunc(driver, f, table)
	generateCountFunc(driver, f, table)
	generateFindFunc(driver, f, table)
	generateQueryFunc(driver, f, table)

	generateDeleteFunc(driver, f, table)

	generateUpdateFunc(driver, f, table)

	log.Debugf("%#v\n", f)
	err := f.Save(fmt.Sprintf("./%s/%s.go", output, table.Name))
	if err != nil {
		log.Errorln("保存文件失败" + err.Error())
	}
}

// generateAddFunc
/* @Description: 生成添加数据的方法
*  @param f
*  @param table
 */
func generateAddFunc(d Driver, f *jen.File, table *Table) {
	f.Comment("Add" + Case2Camel(table.Name))
	f.Comment("添加数据")
	f.Func().Id("Add"+Case2Camel(table.Name)).Params(jen.Id(table.Name).Id(Case2Camel(table.Name))).Error().Block(
		jen.Id("ping").Call(),
		jen.List(jen.Id("_"), jen.Id("err")).Op(":=").Id("db.Exec").CallFunc(func(group *jen.Group) {
			var fields []string
			var datas []string
			for _, field := range table.Fields {
				if !field.isAuto {
					fields = append(fields, field.fieldName)
					datas = append(datas, "?")
				}
			}
			insertSql := fmt.Sprintf(`insert into %s (%s) values (%s)`, table.Name, strings.Join(fields, ","), strings.Join(datas, ","))
			group.Lit(insertSql)
			for _, field := range table.Fields {
				if !field.isAuto {
					group.Id(table.Name + "." + Case2Camel(field.fieldName))
				}
			}
		}),
		jen.Return(jen.Id("err")),
	)
}

// generateCountFunc
/* @Description: 生成查询数据条数的方法
*  @param f
*  @param table
 */
func generateCountFunc(d Driver, f *jen.File, table *Table) {
	f.Comment("Count" + Case2Camel(table.Name))
	f.Comment("查询数据条数")
	f.Func().Id("Count"+Case2Camel(table.Name)).Params(jen.Id("condition").String()).Int().Block(
		jen.Id("ping").Call(),
		jen.Var().Id("count").Int(),
		jen.List(jen.Id("err")).Op(":=").Id("db.QueryRow").CallFunc(func(group *jen.Group) {
			countSql := fmt.Sprintf("select count(*) from %s where ", table.Name)
			group.Lit(countSql).Op("+").Id("condition")
		}).Op(".").Id("Scan").Call(jen.Op("&").Id("count")),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Qual("github.com/sirupsen/logrus", "Errorln").Call(jen.Lit("查询数据库出现错误"), jen.Id("err.Error").Call()),
			jen.Return(jen.Id("0")),
		),
		jen.Return(jen.Id("count")),
	)
}

// generateFindFunc
/* @Description: 生成根据id查找的方法
*  @param d
*  @param f
*  @param table
 */
func generateFindFunc(d Driver, f *jen.File, table *Table) {
	f.Comment("Find" + Case2Camel(table.Name))
	f.Comment("查询一条数据")
	f.Func().
		// 方法名
		Id("Find"+Case2Camel(table.Name)).
		// 参数列表
		Params(d.ConvertType(jen.Id(table.Pri.fieldName), table.Pri.fieldName)).
		// 返回值，多个参数用call,打括号
		Call(jen.Op("*").Id(Case2Camel(table.Name)), jen.Error()).
		// 方法体
		Block(
			jen.Id("ping").Call(),
			jen.Id("bean").Op(":=").New(jen.Id(Case2Camel(table.Name))),
			jen.List(jen.Id("err")).Op(":=").Id("db.QueryRow").CallFunc(func(group *jen.Group) {
				countSql := fmt.Sprintf("select * from %s  where %s=?", table.Name, table.Pri.fieldName)
				group.Lit(countSql)
				group.Id(table.Pri.fieldName)
			}).Op(".").Id("Scan").CallFunc(func(group *jen.Group) {
				for _, field := range table.Fields {
					group.Op("&").Id("bean." + Case2Camel(field.fieldName))
				}
			}),

			jen.Return(jen.Id("bean"), jen.Id("err")),
		)
}

// generateQueryFunc
/* @Description: 生成按条件查询多个的方法
*  @param d
*  @param f
*  @param table
 */
func generateQueryFunc(d Driver, f *jen.File, table *Table) {
	f.Comment("Query" + Case2Camel(table.Name))
	f.Comment("根据条件查询多条数据")
	f.Func().
		// 方法名
		Id("Query"+Case2Camel(table.Name)).
		// 参数列表
		Params(jen.Id("condition").String()).
		// 返回值，多个参数用call,打括号
		Call(jen.Op("[]").Op("*").Id(Case2Camel(table.Name)), jen.Error()).
		// 方法体
		Block(
			jen.Id("ping").Call(),
			// var beans []*Beans
			jen.Var().Id("beans").Op("[]*").Id(Case2Camel(table.Name)),
			jen.List(jen.Id("results"), jen.Id("err")).Op(":=").Id("db.Query").CallFunc(func(group *jen.Group) {
				countSql := fmt.Sprintf("select * from %s  where ", table.Name)
				group.Lit(countSql).Op("+").Id("condition")
			}),
			jen.If(jen.Id("err").Op("!=").Nil()).Block(
				jen.Return(jen.Id("beans"), jen.Id("err")),
			),

			jen.For(jen.Id("results.Next").Call()).Block(
				jen.Id("bean").Op(":=").New(jen.Id(Case2Camel(table.Name))),
				jen.Id("err").Op(":=").Id("results.Scan").CallFunc(func(group *jen.Group) {
					for _, field := range table.Fields {
						group.Op("&").Id("bean." + Case2Camel(field.fieldName))
					}
				}),
				jen.If(jen.Id("err").Op("!=").Nil()).Block(
					jen.Qual("github.com/sirupsen/logrus", "Errorln").Call(jen.Lit("查询数据库出现错误"), jen.Id("err.Error").Call()),
					jen.Id("_").Op("=").Id("results.Close").Call(),
					jen.Return(jen.Id("beans"), jen.Id("err")),
				),
				jen.Id("beans").Op("=").Append(jen.Id("beans"), jen.Id("bean")),
			),

			jen.Return(jen.Id("beans"), jen.Id("err")),
		)
}

func generateDeleteFunc(d Driver, f *jen.File, table *Table) {
	f.Comment("Delete" + Case2Camel(table.Name))
	f.Comment("根据主键删除一条数据")
	f.Func().
		// 方法名
		Id("Delete"+Case2Camel(table.Name)).
		// 参数列表
		Params(d.ConvertType(jen.Id(table.Pri.fieldName), table.Pri.fieldName)).Error().
		// 返回值，多个参数用call,打括号
		// 方法体
		Block(
			jen.Id("ping").Call(),
			jen.List(jen.Id("_"), jen.Id("err")).Op(":=").Id("db.Exec").CallFunc(func(group *jen.Group) {
				countSql := fmt.Sprintf("delete from %s  where %s=?", table.Name, table.Pri.fieldName)
				group.Lit(countSql)
				group.Id(table.Pri.fieldName)
			}),

			jen.Return(jen.Id("err")),
		)
}

func generateUpdateFunc(d Driver, f *jen.File, table *Table) {
	f.Comment("Update" + Case2Camel(table.Name))
	f.Comment("根据主键更新一条数据")
	f.Func().
		// 方法名
		Id("Update"+Case2Camel(table.Name)).
		// 参数列表
		Params(jen.Id(table.Name).Op("*").Id(Case2Camel(table.Name))).Error().
		// 返回值，多个参数用call,打括号
		// 方法体
		Block(
			jen.Id("ping").Call(),
			jen.List(jen.Id("_"), jen.Id("err")).Op(":=").Id("db.Exec").CallFunc(func(group *jen.Group) {
				var fields []string

				for _, field := range table.Fields {
					if field.fieldName != table.Pri.fieldName {
						fields = append(fields, field.fieldName+"=?")
					}
				}
				countSql := fmt.Sprintf("update %s set %s where %s=?", table.Name, strings.Join(fields, ","), table.Pri.fieldName)
				group.Lit(countSql)
				for _, field := range table.Fields {
					if field.fieldName != table.Pri.fieldName {
						group.Id(table.Name + "." + Case2Camel(field.fieldName))
					}
				}
				group.Id(table.Name + "." + Case2Camel(table.Pri.fieldName))
			}),

			jen.Return(jen.Id("err")),
		)
}
