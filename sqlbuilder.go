// Copyright 2014 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/issue9/orm/core"
	"github.com/issue9/orm/sqlbuilder"
)

func getModel(v interface{}) (*core.Model, reflect.Value, error) {
	m, err := core.NewModel(v)
	if err != nil {
		return nil, reflect.Value{}, err
	}

	rval := reflect.ValueOf(v)
	for rval.Kind() == reflect.Ptr {
		rval = rval.Elem()
	}

	return m, rval, nil
}

// 根据 model 中的主键或是唯一索引为 sql 产生 where 语句，
// 若两者都不存在，则返回错误信息。rval 为 struct 的 reflect.Value
func where(sql sqlbuilder.WhereStmter, m *core.Model, rval reflect.Value) error {
	vals := make([]interface{}, 0, 3)
	keys := make([]string, 0, 3)

	// 获取构成 where 的键名和键值
	getKV := func(cols []*core.Column) bool {
		if len(cols) == 0 {
			return false
		}

		for _, col := range cols {
			field := rval.FieldByName(col.GoName)

			if !field.IsValid() ||
				col.Zero == field.Interface() {
				vals = vals[:0]
				keys = keys[:0]
				return false
			}

			keys = append(keys, col.Name)
			vals = append(vals, field.Interface())
		}
		return true
	}

	if !getKV(m.PK) { // 没有主键，则尝试唯一约束
		for _, cols := range m.UniqueIndexes {
			if getKV(cols) {
				break
			}
		}
	}

	if len(keys) == 0 {
		return fmt.Errorf("没有主键或唯一约束，无法为 %s 产生 where 部分语句", m.Name)
	}

	for index, key := range keys {
		sql.WhereStmt().And("{"+key+"}=?", vals[index])
	}

	return nil
}

// 根据 rval 中任意非零值产生 where 语句
func whereAny(sql sqlbuilder.WhereStmter, m *core.Model, rval reflect.Value) error {
	vals := make([]interface{}, 0, 3)
	keys := make([]string, 0, 3)

	for _, col := range m.Cols {
		field := rval.FieldByName(col.GoName)

		if !field.IsValid() || col.Zero == field.Interface() {
			continue
		}

		keys = append(keys, col.Name)
		vals = append(vals, field.Interface())
	}

	if len(keys) == 0 {
		return fmt.Errorf("没有非零值字段，无法为 %s 产生 where 部分语句", m.Name)
	}

	for index, key := range keys {
		sql.WhereStmt().And("{"+key+"}=?", vals[index])
	}

	return nil
}

// 统计符合 v 条件的记录数量。
func count(e core.Engine, v interface{}) (int64, error) {
	m, rval, err := getModel(v)
	if err != nil {
		return 0, err
	}

	sql := sqlbuilder.Select(e).Count("COUNT(*) AS count").From("{#" + m.Name + "}")
	if err = whereAny(sql, m, rval); err != nil {
		return 0, err
	}

	return sql.QueryInt("count")
}

// 创建表。可能有多条执行语句，所以只能是事务。
func create(e *Tx, v interface{}) error {
	m, _, err := getModel(v)
	if err != nil {
		return err
	}

	sql := core.NewStringBuilder("")
	if err = e.Dialect().CreateTableSQL(sql, m); err != nil {
		return err
	}
	if _, err := e.Exec(sql.String()); err != nil {
		return err
	}

	// CREATE INDEX，部分数据库并没有直接有 create table with index 功能
	for name, cols := range m.KeyIndexes {
		sql.Reset().
			WriteString("CREATE INDEX ").
			WriteByte('{').WriteString(name).WriteByte('}').
			WriteString(" ON ").
			WriteString("{#").WriteString(m.Name).WriteString("}(")
		for _, col := range cols {
			sql.WriteByte('{').WriteString(col.Name).WriteString("},")
		}
		sql.TruncateLast(1)
		sql.WriteByte(')')
		if _, err := e.Exec(sql.String()); err != nil {
			return err
		}
	}
	return nil
}

// 删除一张表。
func drop(e core.Engine, v interface{}) error {
	m, err := core.NewModel(v)
	if err != nil {
		return err
	}

	_, err = sqlbuilder.DropTable(e, "{#"+m.Name+"}").Exec()
	return err
}

// 清空表，并重置 AI 计数。
// 系统会默认给表名加上表名前缀。
func truncate(e core.Engine, v interface{}) error {
	m, err := core.NewModel(v)
	if err != nil {
		return err
	}

	aiName := ""
	if m.AI != nil {
		aiName = m.AI.Name
	}

	sql := e.Dialect().TruncateTableSQL("#"+m.Name, aiName)
	_, err = e.Exec(sql)
	return err
}

func insert(e core.Engine, v interface{}) (sql.Result, error) {
	m, rval, err := getModel(v)
	if err != nil {
		return nil, err
	}

	sql := sqlbuilder.Insert(e, "{#"+m.Name+"}")
	for name, col := range m.Cols {
		field := rval.FieldByName(col.GoName)
		if !field.IsValid() {
			return nil, fmt.Errorf("未找到该名称 %s 的值", col.GoName)
		}

		// 在为零值的情况下，若该列是 AI 或是有默认值，则过滤掉。无论该零值是否为手动设置的。
		if col.Zero == field.Interface() &&
			(col.IsAI() || col.HasDefault) {
			continue
		}

		sql.KeyValue("{"+name+"}", field.Interface())
	}

	return sql.Exec()
}

// 查找数据。
//
// 根据 v 的 pk 或中唯一索引列查找一行数据，并赋值给 v。
// 若 v 为空，则不发生任何操作，v 可以是数组。
func find(e core.Engine, v interface{}) error {
	m, rval, err := getModel(v)
	if err != nil {
		return err
	}

	sql := sqlbuilder.Select(e).
		Select("*").
		From("{#" + m.Name + "}")
	if err = where(sql, m, rval); err != nil {
		return err
	}

	_, err = sql.QueryObj(v)
	return err
}

// for update 只能作用于事务
func forUpdate(tx *Tx, v interface{}) error {
	m, rval, err := getModel(v)
	if err != nil {
		return err
	}

	sql := sqlbuilder.Select(tx).
		Select("*").
		From("{#" + m.Name + "}").
		ForUpdate()
	if err = where(sql, m, rval); err != nil {
		return err
	}

	_, err = sql.QueryObj(v)
	return err
}

// 更新 v 到数据库，默认情况下不更新零值。
// cols 表示必须要更新的列，即使是零值。
//
// 更新依据为每个对象的主键或是唯一索引列。
// 若不存在此两个类型的字段，则返回错误信息。
func update(e core.Engine, v interface{}, cols ...string) (sql.Result, error) {
	m, rval, err := getModel(v)
	if err != nil {
		return nil, err
	}

	sql := sqlbuilder.Update(e, "{#"+m.Name+"}")
	for name, col := range m.Cols {
		field := rval.FieldByName(col.GoName)
		if !field.IsValid() {
			return nil, fmt.Errorf("未找到该名称 %s 的值", col.GoName)
		}

		// 零值，但是不属于指定需要更新的列
		if !inStrSlice(name, cols) && col.Zero == field.Interface() {
			continue
		}

		sql.Set("{"+name+"}", field.Interface())
	}

	if err := where(sql, m, rval); err != nil {
		return nil, err
	}

	return sql.Exec()
}

func inStrSlice(key string, slice []string) bool {
	for _, v := range slice {
		if v == key {
			return true
		}
	}
	return false
}

// 将 v 生成 delete 的 sql 语句
func del(e core.Engine, v interface{}) (sql.Result, error) {
	m, rval, err := getModel(v)
	if err != nil {
		return nil, err
	}

	sql := sqlbuilder.Delete(e, "{#"+m.Name+"}")
	if err = where(sql, m, rval); err != nil {
		return nil, err
	}

	return sql.Exec()
}

// rval 为结构体指针组成的数据
func buildInsertManySQL(e *Tx, rval reflect.Value) (*sqlbuilder.InsertStmt, error) {
	sql := sqlbuilder.Insert(e, "")
	keys := []string{}         // 保存列的顺序，方便后续元素获取值
	var firstType reflect.Type // 记录数组中第一个元素的类型，保证后面的都相同

	for i := 0; i < rval.Len(); i++ {
		irval := rval.Index(i)

		m, irval, err := getModel(irval.Interface())
		if err != nil {
			return nil, err
		}

		if i == 0 { // 第一个元素，需要从中获取列信息。
			firstType = irval.Type()
			sql.Table("{#" + m.Name + "}")

			for name, col := range m.Cols {
				field := irval.FieldByName(col.GoName)
				if !field.IsValid() {
					return nil, fmt.Errorf("未找到该名称 %s 的值", col.GoName)
				}

				// 在为零值的情况下，若该列是 AI 或是有默认值，则过滤掉。无论该零值是否为手动设置的。
				if col.Zero == field.Interface() &&
					(col.IsAI() || col.HasDefault) {
					continue
				}

				sql.KeyValue("{"+name+"}", field.Interface())
				keys = append(keys, name)
			}
		} else { // 之后的元素，只需要获取其对应的值就行
			if firstType != irval.Type() { // 与第一个元素的类型不同。
				return nil, errors.New("参数 v 中包含了不同类型的元素")
			}

			vals := make([]interface{}, 0, len(keys))
			for _, name := range keys {
				col, found := m.Cols[name]
				if !found {
					return nil, fmt.Errorf("不存在的列名 %s", name)
				}

				field := irval.FieldByName(col.GoName)
				if !field.IsValid() {
					return nil, fmt.Errorf("未找到该名称 %s 的值", col.GoName)
				}

				// 在为零值的情况下，若该列是 AI 或是有默认值，则过滤掉。无论该零值是否为手动设置的。
				if col.Zero == field.Interface() &&
					(col.IsAI() || col.HasDefault) {
					continue
				}

				vals = append(vals, field.Interface())
			}
			sql.Values(vals...)
		}
	} // end for array

	return sql, nil
}
