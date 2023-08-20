/*
将 Go 语言的类型映射为数据库中的类型
用来屏蔽不同数据库的差异，实现数据库的统一操作
不同的数据库有不同的实现，所以定义为接口
*/
package dialect

import "reflect"

//定义一个全局的变量，用来存储不同数据库的实现
var dialectsMap = map[string]Dialect{}

type Dialect interface {
	// 将 Go 语言的类型转换为该数据库的数据类型
	DataTypeOf(typ reflect.Value) string
	// 返回某个表是否存在的 SQL 语句，参数是表名(table)
	TableExistSQL(tableName string) (string, []interface{})
}

//setter
func RegisterDialect(name string, dialect Dialect) {
	dialectsMap[name] = dialect
}

//getter
func GetDialect(name string) (dialect Dialect, ok bool) {
	dialect, ok = dialectsMap[name]
	return
}
