package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJsonColumn(t *testing.T) {
	d := map[string]string{
		"key": "value",
	}
	col := NewJSONColumn(&d)
	bs, err := json.Marshal(col)
	assert.Nil(t, err)
	var col1 JSONColumn[map[string]string]
	err = json.Unmarshal(bs, &col1)
	assert.Nil(t, err)
	d2 := *col1.Get()
	assert.Len(t, d2, 1)
	assert.Equal(t, "value", d2["key"])
}

func TestName(t *testing.T) {
	img := &Image{}
	assert.Equal(t, img.TableName(), "image")
	assert.Equal(t, ((*Image)(nil)).TableName(), "image")
}
