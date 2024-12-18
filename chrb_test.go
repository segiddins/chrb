package chrb_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/segiddins/chrb"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var config *chrb.Config

func TestChrb(t *testing.T) {
	config = &chrb.Config{
		Fs: afero.NewMemMapFs(),
		Env: chrb.ParseEnv([]string{
			"HOME=/Users/user",
		}),
		Uid:     1,
		Options: chrb.DefaultOptions.Clone(),
		RubyEnvFinder: func(r *chrb.Ruby) ([]string, error) {
			return []string{
				"RUBY_ROOT=" + string(r.RubyDir),
				"GEM_ROOT=/gem/root",
				"GEM_PATH=/gem/path",
				"GEM_HOME=/gem/home",
				"RUBY_VERSION=" + r.Version,
				// "RUBY_API_VERSION=" + r.Version,
				"RUBY_ENGINE=" + r.Engine,
			}, nil
		},
	}

	for _, dir := range []string{
		"/opt/rubies/ruby-3.1.1",
		"/opt/rubies/ruby-3.1.16",
		"/opt/rubies/ruby-3.3.6",
		"/opt/rubies/3.2.1",
		"/opt/rubies/jruby-9.4.3.0",
		"/opt/rubies/jruby-9.4.8.0",
		"/opt/rubies/jruby-9.5.0.0",
		"/opt/rubies/truffleruby-23.1.0",
		"/opt/rubies/truffleruby-24.0.0",
	} {
		err := config.Fs.MkdirAll(dir+"/bin", 0755)
		if err != nil {
			t.Fatal(err)
		}
		err = afero.WriteFile(config.Fs, filepath.Join(dir, "bin", "ruby"), []byte("ruby"), 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	os.Clearenv()

	rubies, err := chrb.ListRubies(config)
	assert.NoError(t, err)
	t.Logf("rubies: %+v", rubies)

	tests := []struct {
		pattern  string
		execPath string
		env      []string
	}{
		{pattern: "3.2.1", execPath: "/opt/rubies/3.2.1/bin/ruby"},
		{pattern: "3.3.6", execPath: "/opt/rubies/ruby-3.3.6/bin/ruby"},
		{pattern: "jruby-9.4.3.0", execPath: "/opt/rubies/jruby-9.4.3.0/bin/ruby"},
		{pattern: "jruby-9", execPath: "/opt/rubies/jruby-9.5.0.0/bin/ruby"},
		{pattern: "truffleruby-23.1.0", execPath: "/opt/rubies/truffleruby-23.1.0/bin/ruby"},
		{pattern: "truffleruby-", execPath: "/opt/rubies/truffleruby-24.0.0/bin/ruby"},
		{pattern: "truffleruby-23.", execPath: "/opt/rubies/truffleruby-23.1.0/bin/ruby"},
		{pattern: "truffleruby", execPath: "/opt/rubies/truffleruby-24.0.0/bin/ruby"},
		{pattern: "3.1.1", execPath: "/opt/rubies/ruby-3.1.1/bin/ruby"},
		{pattern: "3.1.16", execPath: "/opt/rubies/ruby-3.1.16/bin/ruby"},
		{pattern: "ruby", execPath: "/opt/rubies/ruby-3.3.6/bin/ruby", env: []string{
			"GEM_HOME=/Users/user/.gem/ruby/3.3.6",
			"GEM_PATH=/Users/user/.gem/ruby/3.3.6:/gem/root",
			"GEM_ROOT=/gem/root",
			"HOME=/Users/user",
			"PATH=/opt/rubies/ruby-3.3.6/bin",
			"RUBY_ENGINE=ruby",
			"RUBY_ROOT=/opt/rubies/ruby-3.3.6",
			"RUBY_VERSION=3.3.6",
		}},
	}

	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			ruby, err := chrb.FindRuby(test.pattern, config)
			t.Logf("ruby: %+v", ruby)
			if assert.NoError(t, err) {
				assert.Equal(t, test.execPath, ruby.ExecPath())
				env, err := ruby.Env(config)
				envList := env.ToEnvList()
				slices.Sort(envList)
				if assert.NoError(t, err) {
					if test.env != nil {
						assert.Equal(t, test.env, envList)
					}
				}
			}
		})
	}

	{
		_, err := chrb.FindRuby("truffleruby-2", config)
		assert.Error(t, err)
	}
}
