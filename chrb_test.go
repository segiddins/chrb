package chrb_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/segiddins/chrb"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestChrb(t *testing.T) {
	mmfs := afero.NewMemMapFs()
	chrb.Fs = mmfs

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
		err := mmfs.MkdirAll(dir+"/bin", 0755)
		if err != nil {
			t.Fatal(err)
		}
		err = afero.WriteFile(mmfs, filepath.Join(dir, "bin", "ruby"), []byte("ruby"), 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	os.Clearenv()

	rubies, err := chrb.ListRubies()
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
		{pattern: "jruby-9", execPath: "/opt/rubies/jruby-9.4.3.0/bin/ruby"},
		{pattern: "truffleruby-23.1.0", execPath: "/opt/rubies/truffleruby-23.1.0/bin/ruby"},
		{pattern: "truffleruby", execPath: "/opt/rubies/truffleruby-24.0.0/bin/ruby"},
		{pattern: "3.1.1", execPath: "/opt/rubies/ruby-3.1.1/bin/ruby"},
		{pattern: "3.1.16", execPath: "/opt/rubies/ruby-3.1.16/bin/ruby"},
	}

	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			ruby, err := chrb.FindRuby(test.pattern)
			if assert.NoError(t, err) {
				assert.Equal(t, test.execPath, ruby.ExecPath())
				if test.env != nil {
					env, err := ruby.Env()
					if assert.NoError(t, err) {
						assert.Equal(t, test.env, env)
					}
				}
			}
		})
	}
}
