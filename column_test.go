// Copyright 2018 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package orm

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/issue9/orm/internal/modeltest"

	"github.com/issue9/assert"
)

func TestColumn_SetLen(t *testing.T) {
	a := assert.New(t)
	col := &Column{}

	a.NotError(col.setLen([]string{})).Equal(col.Len1, 0).Equal(col.Len2, 0)
	a.NotError(col.setLen([]string{"1", "2"})).Equal(col.Len1, 1).Equal(col.Len2, 2)
	a.Error(col.setLen([]string{"1", "2", "3"}))
	a.Error(col.setLen([]string{"1", "one"}))
	a.Error(col.setLen([]string{"one", "one"}))
}

func TestColumn_IsZero(t *testing.T) {
	a := assert.New(t)
	col := &Column{}

	col.GoType = reflect.TypeOf(int(5))
	col.zero = reflect.Zero(col.GoType).Interface()
	a.True(col.IsZero(reflect.ValueOf(int(0))))
	a.False(col.IsZero(reflect.ValueOf(1)))

	col.GoType = reflect.TypeOf([]byte{})
	col.zero = reflect.Zero(col.GoType).Interface()
	a.True(col.IsZero(reflect.ValueOf([]byte{})))
	a.True(col.IsZero(reflect.ValueOf([]byte(""))))
	a.False(col.IsZero(reflect.ValueOf([]byte{'0'})))

	col.GoType = reflect.TypeOf(sql.RawBytes{})
	col.zero = reflect.Zero(col.GoType).Interface()
	a.True(col.IsZero(reflect.ValueOf([]byte{})))
	a.True(col.IsZero(reflect.ValueOf([]byte(""))))
	a.False(col.IsZero(reflect.ValueOf([]byte{'0'})))
}

func TestColumn_SetNullable(t *testing.T) {
	a := assert.New(t)

	col := &Column{}

	a.False(col.Nullable)
	a.NotError(col.setNullable([]string{})).True(col.Nullable)
	a.NotError(col.setNullable([]string{"false"})).False(col.Nullable)
	a.NotError(col.setNullable([]string{"T"})).True(col.Nullable)
	a.NotError(col.setNullable([]string{"0"})).False(col.Nullable)

	a.Error(col.setNullable([]string{"1", "2"}))
	a.Error(col.setNullable([]string{"T1"}))

	// 将 AI 设置为 nullabl
	m, err := NewModel(&modeltest.User{})
	a.NotError(err).NotNil(m)
	m.AI = col
	col.model = m
	a.Error(col.setNullable([]string{"true"}))
}
