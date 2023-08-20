package session

import (
	"testing"
)

// var (
// 	TestDB      *sql.DB
// 	TestDial, _ = dialect.GetDialect("sqlite3")
// )

type User struct {
	Name string `geeorm:"PRIMARY KEY"`
	Age  int
}

func TestSession_CreateTable(t *testing.T) {
	s := NewSession().Model(&User{})
	_ = s.DropTable()
	_ = s.CreateTable()
	if !s.HasTable() {
		t.Fatal("Failed to create table User")
	}
}

// func NewSession() *Session {
// 	return New(TestDB, TestDial)
// }
