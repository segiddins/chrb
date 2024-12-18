package chrb

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Env struct {
	RubyRoot       *string
	RubyEngine     *string
	RubyVersion    *string
	RubyApiVersion *string
	RubyOpt        *string
	GemRoot        *string
	GemPath        *string
	GemHome        *string

	Home   *string
	Path   *string
	Prefix *string

	Rest map[string]*string
}

var specialEnvKeys = []string{
	"RUBY_ROOT",
	"RUBY_ENGINE",
	"RUBY_VERSION",
	"RUBY_API_VERSION",
	"RUBYOPT",
	"GEM_ROOT",
	"GEM_PATH",
	"GEM_HOME",

	"HOME",
	"PATH",
	"PREFIX",
}

func ParseEnv(envList []string) *Env {
	envMap := make(map[string]*string, len(envList))
	for _, e := range envList {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envMap[parts[0]] = &parts[1]
	}

	env := Env{
		RubyRoot:       envMap["RUBY_ROOT"],
		RubyEngine:     envMap["RUBY_ENGINE"],
		RubyVersion:    envMap["RUBY_VERSION"],
		RubyApiVersion: envMap["RUBY_API_VERSION"],
		RubyOpt:        envMap["RUBYOPT"],
		GemRoot:        envMap["GEM_ROOT"],
		GemPath:        envMap["GEM_PATH"],
		GemHome:        envMap["GEM_HOME"],

		Home:   envMap["HOME"],
		Path:   envMap["PATH"],
		Prefix: envMap["PREFIX"],

		Rest: envMap,
	}

	return &env
}

func (e *Env) LookupEnv(key string) (string, bool) {
	f := func(s *string) (string, bool) {
		if s == nil {
			return "", false
		}
		return *s, true
	}

	switch key {
	case "RUBY_ROOT":
		return f(e.RubyRoot)
	case "RUBY_ENGINE":
		return f(e.RubyEngine)
	case "RUBY_VERSION":
		return f(e.RubyVersion)
	case "RUBY_API_VERSION":
		return f(e.RubyApiVersion)
	case "RUBYOPT":
		return f(e.RubyOpt)
	case "GEM_ROOT":
		return f(e.GemRoot)
	case "GEM_PATH":
		return f(e.GemPath)
	case "GEM_HOME":
		return f(e.GemHome)
	case "HOME":
		return f(e.Home)
	case "PATH":
		return f(e.Path)
	case "PREFIX":
		return f(e.Prefix)
	}

	v, ok := e.Rest[key]
	if !ok {
		return "", false
	}
	return *v, true
}

func (e *Env) Getenv(key string) string {
	v, _ := e.LookupEnv(key)
	return v
}

func (e *Env) ToEnvList() []string {
	envList := []string{}
	if e.RubyRoot != nil {
		envList = append(envList, fmt.Sprintf("RUBY_ROOT=%s", *e.RubyRoot))
	}
	if e.RubyEngine != nil {
		envList = append(envList, fmt.Sprintf("RUBY_ENGINE=%s", *e.RubyEngine))
	}
	if e.RubyVersion != nil {
		envList = append(envList, fmt.Sprintf("RUBY_VERSION=%s", *e.RubyVersion))
	}
	if e.RubyApiVersion != nil {
		envList = append(envList, fmt.Sprintf("RUBY_API_VERSION=%s", *e.RubyApiVersion))
	}
	if e.RubyOpt != nil {
		envList = append(envList, fmt.Sprintf("RUBYOPT=%s", *e.RubyOpt))
	}
	if e.GemRoot != nil {
		envList = append(envList, fmt.Sprintf("GEM_ROOT=%s", *e.GemRoot))
	}
	if e.GemPath != nil {
		envList = append(envList, fmt.Sprintf("GEM_PATH=%s", *e.GemPath))
	}
	if e.GemHome != nil {
		envList = append(envList, fmt.Sprintf("GEM_HOME=%s", *e.GemHome))
	}
	if e.Home != nil {
		envList = append(envList, fmt.Sprintf("HOME=%s", *e.Home))
	}
	if e.Path != nil {
		envList = append(envList, fmt.Sprintf("PATH=%s", *e.Path))
	}
	if e.Prefix != nil {
		envList = append(envList, fmt.Sprintf("PREFIX=%s", *e.Prefix))
	}
	for k, v := range e.Rest {
		if slices.Contains(specialEnvKeys, k) {
			continue
		}
		envList = append(envList, fmt.Sprintf("%s=%s", k, *v))
	}
	return envList
}

func (e *Env) ResetRubyEnv(uid int) {
	rubyRoot := e.Getenv("RUBY_ROOT")
	if len(rubyRoot) == 0 {
		return
	}

	path := filepath.SplitList(e.Getenv("PATH"))
	path = deleteElement(path, filepath.Join(rubyRoot, "bin"))

	gemRoot := e.Getenv("GEM_ROOT")
	if len(gemRoot) > 0 {
		path = deleteElement(path, filepath.Join(gemRoot, "bin"))
	}

	if uid != 0 {
		gemHome := e.Getenv("GEM_HOME")
		gemPath := filepath.SplitList(e.Getenv("GEM_PATH"))
		gemRoot := e.Getenv("GEM_ROOT")

		if len(gemHome) > 0 {
			path = deleteElement(path, filepath.Join(gemHome, "bin"))
			gemPath = deleteElement(gemPath, gemHome)
		}

		if len(gemRoot) > 0 {
			gemPath = deleteElement(gemPath, gemRoot)
		}

		if len(gemPath) > 0 {
			gemPathString := strings.Join(gemPath, string(os.PathListSeparator))
			e.GemPath = &gemPathString
		} else {
			e.GemPath = nil
		}
		e.GemHome = nil
	}

	pathString := strings.Join(path, string(os.PathListSeparator))
	e.Path = &pathString

	e.RubyRoot = nil
	e.RubyEngine = nil
	e.RubyVersion = nil
	e.RubyApiVersion = nil
	e.RubyOpt = nil
	e.GemRoot = nil
}

func (e *Env) ExpandEnv(path string) string {
	return os.Expand(path, func(key string) string {
		return e.Getenv(key)
	})
}

func (e *Env) Clone() *Env {
	envList := e.ToEnvList()
	return ParseEnv(envList)
}

func (e *Env) Merge(envList []string) *Env {
	combined := e.ToEnvList()
	combined = append(combined, envList...)
	return ParseEnv(combined)
}

func deleteElement[S ~[]E, E comparable](s S, e E) S {
	return slices.DeleteFunc(s, func(s E) bool {
		return s == e
	})
}

func (e *Env) Diff(list []string) []struct {
	Key   string
	Value *string
} {
	m := make(map[string]string)
	for _, e := range e.ToEnvList() {
		parts := strings.SplitN(e, "=", 2)
		m[parts[0]] = parts[1]
	}

	o := make(map[string]string)
	for _, e := range list {
		parts := strings.SplitN(e, "=", 2)
		o[parts[0]] = parts[1]
	}

	diff := []struct {
		Key   string
		Value *string
	}{}

	for k, v := range m {
		if o[k] != v {
			diff = append(diff, struct {
				Key   string
				Value *string
			}{Key: k, Value: &v})
		}
	}

	for k := range o {
		if _, ok := m[k]; !ok {
			diff = append(diff, struct {
				Key   string
				Value *string
			}{Key: k, Value: nil})
		}
	}

	return diff
}
