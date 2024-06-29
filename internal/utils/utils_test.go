package utils

import (
	"testing"

	"github.com/projecteru2/vmihub/internal/utils/idgen"
	"github.com/stretchr/testify/assert"
)

func TestUniqueSID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10000; i++ {
		id := idgen.NextSID()
		_, ok := seen[id]
		assert.False(t, ok)
		seen[id] = true
	}
}

func TestRoundMoney(t *testing.T) {
	cases := []struct {
		input float64
		res   float64
	}{
		{
			10.1234,
			10.12,
		},
		{
			10.9876,
			10.99,
		},
	}
	for _, c := range cases {
		actual := RoundMoney(c.input)
		assert.Equal(t, c.res, actual)
	}
}
