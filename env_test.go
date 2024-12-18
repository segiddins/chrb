package chrb_test

import (
	"sort"
	"testing"

	"github.com/segiddins/chrb"
	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	env := chrb.ParseEnv([]string{
		"GEM_HOME=/Users/jason/src/chrb/test/gem",
		"GEM_PATH=/Users/jason/src/chrb/test/gem",
		"GEM_ROOT=/Users/jason/src/chrb/test/gem",
		"PATH=/Users/jason/src/chrb/test/gem",
		"HOME=/Users/jason/src/chrb/test/gem",
		"PREFIX=/Users/jason/src/chrb/test/gem",
		"RANDOMa=123",
	})

	assert.Equal(t, []string{
		"GEM_ROOT=/Users/jason/src/chrb/test/gem",
		"GEM_PATH=/Users/jason/src/chrb/test/gem",
		"GEM_HOME=/Users/jason/src/chrb/test/gem",
		"HOME=/Users/jason/src/chrb/test/gem",
		"PATH=/Users/jason/src/chrb/test/gem",
		"PREFIX=/Users/jason/src/chrb/test/gem",
		"RANDOMa=123",
	}, env.ToEnvList())
}

func TestEnv_ResetRubyEnv(t *testing.T) {
	env := chrb.ParseEnv([]string{
		"GEM_HOME=/Users/jason/src/chrb/test/gem",
		"GEM_PATH=/Users/jason/src/chrb/test/gem",
		"GEM_ROOT=/Users/jason/src/chrb/test/gem",
		"PATH=/Users/jason/src/chrb/test/gem",
	})

	env.ResetRubyEnv(0)

	assert.Equal(t, []string{
		"GEM_ROOT=/Users/jason/src/chrb/test/gem",
		"GEM_PATH=/Users/jason/src/chrb/test/gem",
		"GEM_HOME=/Users/jason/src/chrb/test/gem",
		"PATH=/Users/jason/src/chrb/test/gem",
	}, env.ToEnvList())
}

func pointer[T any](v T) *T {
	return &v
}

func TestEnv_Diff(t *testing.T) {
	testCases := []struct {
		original []string
		envList  []string
		diff     []struct {
			Key   string
			Value *string
		}
	}{
		{
			original: []string{"GEM_HOME=/Users/jason/src/chrb/test/gem", "GEM_PATH=/Users/jason/src/chrb/test/gem", "GEM_ROOT=/Users/jason/src/chrb/test/gem", "PATH=/Users/jason/src/chrb/test/gem"},
			envList:  []string{"GEM_HOME=/Users/jason/src/chrb/test/gem", "GEM_PATH=/Users/jason/src/chrb/test/gem", "GEM_ROOT=/Users/jason/src/chrb/test/gem", "PATH=/Users/jason/src/chrb/test/gem"},
			diff: []struct {
				Key   string
				Value *string
			}{},
		},
		{
			original: []string{"GEM_HOME=/Users/jason/src/chrb/test/gem", "GEM_PATH=/Users/jason/src/chrb/test/gem", "GEM_ROOT=/Users/jason/src/chrb/test/gem", "PATH=/Users/jason/src/chrb/test/gem"},
			envList:  []string{"GEM_HOME=/Users/jason/src/chrb/test/gem", "GEM_PATH=/Users/jason/src/chrb/test/gem", "GEM_ROOT=/Users/jason/src/chrb/test/gem", "PATH=/Users/jason/src/chrb/test/gem"},
			diff: []struct {
				Key   string
				Value *string
			}{},
		},
		{
			original: []string{"A=b", "C=d", "E=f"},
			envList:  []string{"G=h", "A=d", "C=d"},
			diff: []struct {
				Key   string
				Value *string
			}{
				{Key: "A", Value: pointer("d")},
				{Key: "E", Value: nil},
				{Key: "G", Value: pointer("h")},
			},
		},
	}

	for _, testCase := range testCases {
		env := chrb.ParseEnv(testCase.envList)
		diff := env.Diff(testCase.original)
		sort.Slice(diff, func(i, j int) bool {
			return diff[i].Key < diff[j].Key
		})
		assert.Equal(t, testCase.diff, diff)
	}
}
