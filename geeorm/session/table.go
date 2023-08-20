//用于放置操作数据库表相关的代码
//拼接相关的 SQL 语句
//使用raw的方法执行 SQL 语句
package session

import (
	"fmt"
	"geeorm/log"
	"geeorm/schema"
	"reflect"
	"strings"
)

func (s *Session) Model(value interface{}) *Session {
	// nil or different model, update refTable
	if s.refTable == nil || reflect.TypeOf(value) != reflect.TypeOf(s.refTable.Model) {
		//解析操作比较耗时，所以将解析的结果保存在成员变量 refTable 中
		s.refTable = schema.Parse(value, s.dialect)
	}
	return s
}

//返回 refTable 的值，如果 refTable 未被赋值，则打印错误日志
func (s *Session) RefTable() *schema.Schema {
	if s.refTable == nil {
		log.Error("Model is not set")
	}
	return s.refTable
}

//创建表
func (s *Session) CreateTable() error {
	table := s.RefTable()
	var columns []string
	//遍历表的字段，将字段名和类型拼接成字符串
	for _, field := range table.Fields {
		columns = append(columns, field.Name+" "+field.Type)
	}
	//拼接创建表的 SQL 语句
	desc := strings.Join(columns, ",")
	_, err := s.Raw(fmt.Sprintf("CREATE TABLE %s (%s);", table.Name, desc)).Exec()
	return err
}

//删除表
func (s *Session) DropTable() error {
	_, err := s.Raw(fmt.Sprintf("DROP TABLE IF EXISTS %s", s.RefTable().Name)).Exec()
	return err
}

//判断表是否存在
func (s *Session) HasTable() bool {
	//获取表名
	sql, values := s.dialect.TableExistSQL(s.RefTable().Name)
	//执行查询语句
	row := s.Raw(sql, values...).QueryRow()
	var tmp string
	//将查询结果赋值给 tmp
	_ = row.Scan(&tmp)
	//判断 tmp 是否为空
	return tmp == s.RefTable().Name
}
