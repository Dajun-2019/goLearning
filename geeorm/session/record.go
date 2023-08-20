//用于放置操作数据库行相关的代码
package session

import (
	"errors"
	"geeorm/clause"
	"reflect"
)

func (s *Session) Insert(values ...interface{}) (int64, error) {
	recordValues := make([]interface{}, 0)
	for _, value := range values {
		s.CallMethod(BeforeInsert, value)
		//解析出来的表结构
		table := s.Model(value).RefTable()
		//调用_select()方法，拼接 SQL 语句
		s.clause.Set(clause.INSERT, table.Name, table.FieldNames)
		recordValues = append(recordValues, table.RecordValues(value))
	}

	s.clause.Set(clause.VALUES, recordValues...)
	sql, vars := s.clause.Build(clause.INSERT, clause.VALUES)
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterInsert, nil)
	return result.RowsAffected()
}

func (s *Session) Find(values interface{}) error {
	s.CallMethod(BeforeQuery, nil)
	//获取切片的反射值
	destSlice := reflect.Indirect(reflect.ValueOf(values))
	//获取切片的元素类型
	destType := destSlice.Type().Elem()
	//获取表结构，使用New创建了一个新的destType实例，然后通过Elem()获取指针指向的值
	table := s.Model(reflect.New(destType).Elem().Interface()).RefTable()

	//拼接sql语句
	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	sql, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.ORDERBY, clause.LIMIT)
	//查询到所有的结果
	rows, err := s.Raw(sql, vars...).QueryRows()
	if err != nil {
		return err
	}

	//遍历查询结果
	for rows.Next() {
		//利用反射创建一个destType类型的实例
		dest := reflect.New(destType).Elem()
		var values []interface{}
		//将dest中的所有字段平铺开，构造切片values
		for _, name := range table.FieldNames {
			values = append(values, dest.FieldByName(name).Addr().Interface())
		}
		//将查询到的结果赋值给dest
		if err := rows.Scan(values...); err != nil {
			return err
		}
		s.CallMethod(AfterQuery, dest.Addr().Interface())
		//将dest添加到destSlice中
		destSlice.Set(reflect.Append(destSlice, dest))
	}
	return rows.Close()
}

//根据传入的参数，更新数据库中的记录, kv的格式为 map[string]interface{}
func (s *Session) Update(kv ...interface{}) (int64, error) {
	s.CallMethod(BeforeUpdate, nil)
	//调用clause.go中的Set()方法，拼接SQL语句
	m, ok := kv[0].(map[string]interface{})
	if !ok {
		//如果不是map类型，就将kv转换为map类型
		m = make(map[string]interface{})
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
	}

	//获取表结构
	s.clause.Set(clause.UPDATE, s.RefTable().Name, m)
	//拼接SQL语句
	sql, vars := s.clause.Build(clause.UPDATE, clause.WHERE)
	//执行SQL语句
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterUpdate, nil)
	return result.RowsAffected()
}

func (s *Session) Delete() (int64, error) {
	s.CallMethod(BeforeDelete, nil)
	//获取表结构
	s.clause.Set(clause.DELETE, s.RefTable().Name)
	//拼接SQL语句
	sql, vars := s.clause.Build(clause.DELETE, clause.WHERE)
	//执行SQL语句
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.CallMethod(AfterDelete, nil)
	return result.RowsAffected()
}

func (s *Session) Count() (int64, error) {
	//获取表结构
	s.clause.Set(clause.COUNT, s.RefTable().Name)
	//拼接SQL语句
	sql, vars := s.clause.Build(clause.COUNT, clause.WHERE)
	//执行SQL语句
	row := s.Raw(sql, vars...).QueryRow()
	var count int64
	//将查询到的结果赋值给count
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

//链式调用，示例：s.Where("Age > 18").Limit(3).Find(&users)

func (s *Session) Limit(num int) *Session {
	//调用clause.go中的Set()方法，拼接SQL语句
	s.clause.Set(clause.LIMIT, num)
	return s
}

func (s *Session) Where(desc string, args ...interface{}) *Session {
	//调用clause.go中的Set()方法，拼接SQL语句
	s.clause.Set(clause.WHERE, append([]interface{}{desc}, args...)...)
	return s
}

func (s *Session) OrderBy(desc string) *Session {
	//调用clause.go中的Set()方法，拼接SQL语句
	s.clause.Set(clause.ORDERBY, desc)
	return s
}

//返回一行数据，通过调用Limit(1)和Find()实现
func (s *Session) First(value interface{}) error {
	dest := reflect.Indirect(reflect.ValueOf(value))
	destSlice := reflect.New(reflect.SliceOf(dest.Type())).Elem()
	if err := s.Limit(1).Find(destSlice.Addr().Interface()); err != nil {
		return err
	}
	if destSlice.Len() == 0 {
		return errors.New("NOT FOUND")
	}
	//将destSlice中的第一个元素赋值给dest
	dest.Set(destSlice.Index(0))
	return nil
}
