package models

import (
	"database/sql/driver"
	"encoding/json"
	"reflect"
)

type JSONColumn[T any] struct {
	V *T
}

func (j *JSONColumn[T]) Scan(src any) error {
	if src == nil {
		j.V = nil
		return nil
	}
	j.V = new(T)
	return json.Unmarshal(src.([]byte), j.V)
}

func (j *JSONColumn[T]) Value() (driver.Value, error) {
	raw, err := json.Marshal(j.V)
	return raw, err
}

func (j *JSONColumn[T]) Get() *T {
	return j.V
}

func NewJSONColumn[T any](v *T) JSONColumn[T] {
	return JSONColumn[T]{
		V: v,
	}
}

func GetColumnNames[T any](obj *T) []string {
	ty := reflect.TypeOf(obj).Elem()
	ans := make([]string, 0, ty.NumField())
	for i := 0; i < ty.NumField(); i++ {
		// 获取字段
		field := ty.Field(i)
		dbTag := field.Tag.Get("db")
		if dbTag == "-" {
			continue
		}
		ans = append(ans, dbTag)
	}
	return ans
}
