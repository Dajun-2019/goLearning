//封装原生db操作，提供更加友好的接口，记录日志
package session

import (
	"database/sql"
	"geeorm/clause"
	"geeorm/dialect"
	"geeorm/log"
	"geeorm/schema"
	"strings"
)

// Session keep a pointer to sql.DB and provides all execution of all
type Session struct {
	// 保存指向 sql.DB 的指针，用于执行 SQL 语句
	db  *sql.DB
	sql strings.Builder
	// 保存 SQL 语句中占位符的值，例如 WHERE name = ? 中的 "Tom"
	sqlVars []interface{}
	// 保存数据库的方言，不同的数据库有不同的方言
	dialect dialect.Dialect
	// 保存表结构信息
	refTable *schema.Schema
	// 保存 SQL 语句中的各个部分
	clause clause.Clause
	tx     *sql.Tx
}

//事务相关的方法，db的最小函数集合
type CommonDB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// CommonDB 接口包含了 Exec()、Query()、QueryRow() 三个方法，这三个方法分别用于执行 SQL 语句，查询多行数据，查询单行数据。
var _ CommonDB = (*sql.DB)(nil)

// sql.Tx 也实现了 CommonDB 接口，因此可以在事务中执行 SQL 语句。
var _ CommonDB = (*sql.Tx)(nil)

func New(db *sql.DB, dialect dialect.Dialect) *Session {
	return &Session{
		db:      db,
		dialect: dialect,
	}
}

func (s *Session) Clear() {
	s.sql.Reset()
	s.sqlVars = nil
	s.clause = clause.Clause{}
}

func (s *Session) DB() CommonDB {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

func (s *Session) Raw(sql string, values ...interface{}) *Session {
	s.sql.WriteString(sql)
	s.sql.WriteString(" ")
	s.sqlVars = append(s.sqlVars, values...)
	return s
}

// 执行完成后，清空 (s *Session).sql 和 (s *Session).sqlVars 两个变量。
// 这样 Session 可以复用，开启一次会话，可以执行多次 SQL
func (s *Session) Exec() (result sql.Result, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if result, err = s.DB().Exec(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return
}

func (s *Session) QueryRow() *sql.Row {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	return s.DB().QueryRow(s.sql.String(), s.sqlVars...)
}

func (s *Session) QueryRows() (rows *sql.Rows, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if rows, err = s.DB().Query(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return
}
