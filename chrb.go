package chrb

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/afero"
)

var Fs = afero.NewOsFs()

type Ruby struct {
	Engine  string `json:"engine"`
	Version string `json:"version"`
	RubyDir `json:"ruby_dir"`
}

type Options struct {
	KnownEngines         []string `json:"known_engines"`
	DirectoryEnvPatterns []string `json:"directory_env_patterns"`
	GemHomeEnvPattern    string   `json:"gem_home_env_pattern"`
}

var DefaultOptions = Options{
	KnownEngines:         []string{"ruby", "jruby", "mruby", "truffleruby-jvm", "truffleruby-native", "truffleruby"},
	DirectoryEnvPatterns: []string{"$PREFIX/opt/rubies", "$HOME/.rubies"},
	// TODO: allow using RUBY_API_VERSION instead of RUBY_VERSION
	GemHomeEnvPattern: "$HOME/.gem/$RUBY_ENGINE/$RUBY_VERSION",
}

func (o *Options) Clone() *Options {
	return &Options{
		KnownEngines:         slices.Clone(o.KnownEngines),
		DirectoryEnvPatterns: slices.Clone(o.DirectoryEnvPatterns),
		GemHomeEnvPattern:    strings.Clone(o.GemHomeEnvPattern),
	}
}

func (o *Options) Merge(other *Options) {
	if len(other.KnownEngines) > 0 {
		o.KnownEngines = other.KnownEngines
	}
	if len(other.DirectoryEnvPatterns) > 0 {
		o.DirectoryEnvPatterns = other.DirectoryEnvPatterns
	}
	if len(other.GemHomeEnvPattern) > 0 {
		o.GemHomeEnvPattern = other.GemHomeEnvPattern
	}
}

type RubyEnvFinder func(r *Ruby) ([]string, error)
type Config struct {
	Options       *Options
	Uid           int
	Fs            afero.Fs
	Env           *Env
	RubyEnvFinder RubyEnvFinder
}

type RubyDir string

func (rubyDir RubyDir) ExecPath() string {
	return filepath.Join(string(rubyDir), "bin", "ruby")
}

func RubyFromDir(config *Config, dir RubyDir) (Ruby, error) {
	rubyDir := string(dir)
	_, err := config.Fs.Stat(rubyDir)
	if err != nil {
		return Ruby{}, err
	}
	basename := filepath.Base(rubyDir)
	parts := strings.Split(basename, "-")
	engine := "ruby"
	for _, e := range config.Options.KnownEngines {
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

func ListRubies(config *Config) (rubies []Ruby, err error) {
	for _, dir := range config.Options.DirectoryEnvPatterns {
		dir = config.Env.ExpandEnv(dir)
		entries, err := afero.ReadDir(config.Fs, dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			stat, err := config.Fs.Stat(filepath.Join(dir, entry.Name()))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting stat for %s: %s\n", entry.Name(), err)
				continue
			}
			if stat.IsDir() {
				rubyDir := RubyDir(filepath.Join(dir, entry.Name()))
				ruby, err := RubyFromDir(config, rubyDir)
				if err != nil {
					return nil, err
				}
				rubies = append(rubies, ruby)
			}
		}
	}

	slices.SortFunc(rubies, func(a, b Ruby) int {
		e := strings.Compare(a.Engine, b.Engine)
		if e != 0 {
			return e
		}
		return strings.Compare(a.Version, b.Version)
	})
	return rubies, nil
}

func FindRuby(pattern string, config *Config) (Ruby, error) {
	rubies, err := ListRubies(config)
	if err != nil {
		return Ruby{}, err
	}

	rubiesByEngine := map[string][]Ruby{}
	for _, ruby := range rubies {
		rubiesByEngine[ruby.Engine] = append(rubiesByEngine[ruby.Engine], ruby)
	}

	var engine, version string

	for _, e := range config.Options.KnownEngines {
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

	if len(version) == 0 {
		return versions[len(versions)-1], nil
	}

	for _, ruby := range versions {
		if ruby.Version == version {
			return ruby, nil
		}
	}

	searchVersion := strings.TrimSuffix(version, ".") + "."
	for _, ruby := range slices.Backward(versions) {
		if strings.HasPrefix(ruby.Version, searchVersion) {
			return ruby, nil
		}
	}

	return Ruby{}, fmt.Errorf("no ruby found for pattern: %s", pattern)
}

func ExecFindEnv(r *Ruby) ([]string, error) {
	cmd := exec.Command(r.ExecPath(), "-rrubygems", "-e", `
		puts "RUBY_ENGINE=#{Object.const_defined?(:RUBY_ENGINE) ? RUBY_ENGINE : 'ruby'}"
		print "\0"
		puts "RUBY_VERSION=#{RUBY_VERSION}"
		print "\0"
		begin; require 'rubygems'; puts "GEM_ROOT=#{Gem.default_dir}"; print "\0" rescue LoadError; end
		begin; require 'rubygems'; puts "RUBY_API_VERSION=#{Gem.ruby_api_version}"; print "\0" rescue LoadError; end
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

func (r *Ruby) Env(config *Config) (*Env, error) {
	env := config.Env.Clone()
	execPath := r.ExecPath()
	stat, err := config.Fs.Stat(execPath)
	if err != nil {
		return nil, err
	}
	if stat.Mode()&0o111 != 0o111 {
		return nil, fmt.Errorf("ruby executable is not executable: %s %o", execPath, stat.Mode())
	}

	foundEnv, err := config.RubyEnvFinder(r)
	if err != nil {
		return nil, fmt.Errorf("failed to find env for ruby at %s: %w", r.ExecPath(), err)
	}
	env = env.Merge(foundEnv)

	path := env.Getenv("PATH")
	if len(path) > 0 {
		path = strings.Join([]string{string(r.RubyDir) + "/bin", path}, string(filepath.ListSeparator))
	} else {
		path = string(r.RubyDir) + "/bin"
	}

	if os.Getuid() != 0 {
		gemHome := env.ExpandEnv(DefaultOptions.GemHomeEnvPattern)
		gemPath := gemHome
		gemRoot := env.Getenv("GEM_ROOT")
		if len(gemRoot) > 0 {
			gemPath = strings.Join([]string{gemHome, gemRoot}, string(filepath.ListSeparator))
		}
		env.GemHome = &gemHome
		env.GemPath = &gemPath
		env.GemRoot = &gemRoot
	}

	rubyRoot := string(r.RubyDir)
	env.RubyRoot = &rubyRoot
	env.Path = &path

	return env, nil
}

func FindRubyVersion(config *Config, dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		isDir, err := afero.DirExists(config.Fs, dir)
		if err != nil {
			return "", err
		}
		if !isDir {
			return "", fmt.Errorf("%s is not a directory", dir)
		}

		content, err := afero.ReadFile(config.Fs, filepath.Join(dir, ".ruby-version"))
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil {
			return strings.TrimSpace(string(content)), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no ruby version file found")
}
