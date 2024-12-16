package chrb

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
)

var Fs = afero.NewOsFs()

type Ruby struct {
	Engine  string
	Version string
	RubyDir
}

var engines = []string{"ruby", "jruby", "mruby", "truffleruby-jvm", "truffleruby-native", "truffleruby"}

type RubyDir string

func (rubyDir RubyDir) ExecPath() string {
	return filepath.Join(string(rubyDir), "bin", "ruby")
}

func RubyFromDir(dir RubyDir) (Ruby, error) {
	rubyDir := string(dir)
	_, err := Fs.Stat(rubyDir)
	if err != nil {
		return Ruby{}, err
	}
	basename := filepath.Base(rubyDir)
	parts := strings.Split(basename, "-")
	engine := "ruby"
	for _, e := range engines {
		if parts[0] == e {
			engine = e
			parts = parts[1:]
			break
		}
	}
	version := ""
	if len(parts) == 0 {
		return Ruby{}, fmt.Errorf("invalid ruby directory: %s", rubyDir)
	} else if len(parts) == 1 {
		version = parts[0]
	} else {
		return Ruby{}, fmt.Errorf("invalid ruby directory: %s", rubyDir)
	}

	return Ruby{
		Engine:  engine,
		Version: version,
		RubyDir: dir,
	}, nil
}

func ListRubies() (rubies []Ruby, err error) {
	prefix := os.Getenv("PREFIX")
	home := os.Getenv("HOME")
	for _, dir := range []string{fmt.Sprintf("%s/opt/rubies", prefix), fmt.Sprintf("%s/.rubies", home)} {
		entries, err := afero.ReadDir(Fs, dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			stat, err := Fs.Stat(filepath.Join(dir, entry.Name()))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting stat for %s: %s\n", entry.Name(), err)
				continue
			}
			if stat.IsDir() {
				rubyDir := RubyDir(filepath.Join(dir, entry.Name()))
				ruby, err := RubyFromDir(rubyDir)
				if err != nil {
					return nil, err
				}
				rubies = append(rubies, ruby)
			}
		}
	}
	return
}

func FindRuby(pattern string) (Ruby, error) {
	rubies, err := ListRubies()
	if err != nil {
		return Ruby{}, err
	}

	rubiesByEngine := map[string][]Ruby{}
	for _, ruby := range rubies {
		rubiesByEngine[ruby.Engine] = append(rubiesByEngine[ruby.Engine], ruby)
	}

	for _, rubies := range rubiesByEngine {
		sort.Slice(rubies, func(i, j int) bool {
			return rubies[i].Version < rubies[j].Version
		})
	}

	var engine, version string

	for _, e := range engines {
		if strings.HasPrefix(pattern, e+"-") {
			engine = e
			version = strings.TrimPrefix(pattern, e+"-")
			break
		}
		if pattern == e {
			engine = e
			break
		}
	}

	if len(engine) == 0 {
		engine = "ruby"
		version = pattern
	}

	versions, ok := rubiesByEngine[engine]
	if !ok {
		return Ruby{}, fmt.Errorf("no rubies found for engine: %s", engine)
	}

	for _, ruby := range versions {
		if ruby.Version == version {
			return ruby, nil
		}
	}

	if len(version) == 0 {
		return versions[len(versions)-1], nil
	}

	for _, ruby := range versions {
		if strings.HasPrefix(ruby.Version, version) {
			return ruby, nil
		}
	}

	return Ruby{}, fmt.Errorf("no ruby found for pattern: %s", pattern)
}

func (r *Ruby) findEnv() ([]string, error) {
	cmd := exec.Command(r.ExecPath(), "-rrubygems", "-e", `
		puts "RUBY_ENGINE=#{Object.const_defined?(:RUBY_ENGINE) ? RUBY_ENGINE : 'ruby'}"
		print "\0"
		puts "RUBY_VERSION=#{RUBY_VERSION}"
		print "\0"
		begin; require 'rubygems'; puts "GEM_ROOT=#{Gem.default_dir}"; print "\0" rescue LoadError; end
	`)
	cmd.Env = append([]string{}, "RUBYGEMS_GEMDEPS=")
	cmd.Stderr = nil
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	parsed := buf.String()
	env := strings.Split(parsed, "\n\x00")
	return env, nil
}

func (r *Ruby) Env() ([]string, error) {
	execPath := r.ExecPath()
	stat, err := Fs.Stat(execPath)
	if err != nil {
		return nil, err
	}
	if stat.Mode()&0o111 != 0o111 {
		return nil, fmt.Errorf("ruby executable is not executable: %s %o", execPath, stat.Mode())
	}

	path := os.Getenv("PATH")
	path = fmt.Sprintf("%s:%s", r.RubyDir+"/bin", path)
	env, err := r.findEnv()
	if err != nil {
		return nil, err
	}
	var gemRoot, rubyEngine, rubyVersion, gemPath, gemHome string
	for _, e := range env {
		if strings.HasPrefix(e, "GEM_ROOT=") {
			gemRoot = strings.TrimPrefix(e, "GEM_ROOT=")
		}
		if strings.HasPrefix(e, "RUBY_ENGINE=") {
			rubyEngine = strings.TrimPrefix(e, "RUBY_ENGINE=")
		}
		if strings.HasPrefix(e, "RUBY_VERSION=") {
			rubyVersion = strings.TrimPrefix(e, "RUBY_VERSION=")
		}
	}
	if len(gemRoot) > 0 {
		path = fmt.Sprintf("%s:%s", gemRoot+"/bin", path)
	}

	if os.Getuid() != 0 {
		home := os.Getenv("HOME")
		gemHome = fmt.Sprintf("%s/.gem/%s/%s", home, rubyEngine, rubyVersion)
		gemPath = gemHome
		if len(gemRoot) > 0 {
			gemPath = fmt.Sprintf("%s:%s", gemHome, gemRoot)
		}
		// if envGemPath := os.Getenv("GEM_PATH"); len(envGemPath) > 0 {
		// 	gemPath = fmt.Sprintf("%s:%s", gemHome, envGemPath)
		// }
	}

	return append([]string{
		fmt.Sprintf("RUBY_ROOT=%s", r.RubyDir),
		fmt.Sprintf("GEM_HOME=%s", gemHome),
		fmt.Sprintf("GEM_PATH=%s", gemPath),
		fmt.Sprintf("PATH=%s", path),
	}, env...), nil
}
