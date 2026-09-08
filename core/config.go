package core

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// WorkspaceConfig is the parsed `zoro.toml`.
type WorkspaceConfig struct {
	// Default is the library opened when no query arg is given (by name).
	Default   string        `toml:"default"`
	Libraries []LibrarySpec `toml:"libraries"`
}

// LibrarySpec is one `[[libraries]]` entry.
type LibrarySpec struct {
	Name   string        `toml:"name"`
	Root   string        `toml:"root"`
	Config LibraryConfig `toml:"config"`
}

// LibraryConfig holds per-library consumption preferences.
// Unknown keys are preserved in Extra (forward compatibility: config may land
// before its implementation does).
type LibraryConfig struct {
	// Preview is a candidate preference: default preview target (markdown / html).
	Preview string
	// AllowExec is a candidate preference: whether `@shell` execution is allowed.
	AllowExec *bool
	// Extra preserves any unknown config key.
	Extra map[string]any
}

// Parsing uses a DTO with the raw config table so unknown keys are captured
// before being split into known fields + Extra.
type workspaceConfigDTO struct {
	Default   string           `toml:"default"`
	Libraries []librarySpecDTO `toml:"libraries"`
}

type librarySpecDTO struct {
	Name   string         `toml:"name"`
	Root   string         `toml:"root"`
	Config map[string]any `toml:"config"`
}

// ParseWorkspaceConfig parses `zoro.toml` text.
func ParseWorkspaceConfig(s string) (WorkspaceConfig, error) {
	var dto workspaceConfigDTO
	if err := toml.Unmarshal([]byte(s), &dto); err != nil {
		return WorkspaceConfig{}, err
	}
	cfg := WorkspaceConfig{Default: dto.Default, Libraries: make([]LibrarySpec, 0, len(dto.Libraries))}
	for _, spec := range dto.Libraries {
		cfg.Libraries = append(cfg.Libraries, LibrarySpec{
			Name:   spec.Name,
			Root:   spec.Root,
			Config: finalizeLibraryConfig(spec.Config),
		})
	}
	return cfg, nil
}

// WorkspaceConfigFromPath reads `zoro.toml` and resolves relative roots
// against the config file's directory.
func WorkspaceConfigFromPath(path string) (WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceConfig{}, err
	}
	cfg, err := ParseWorkspaceConfig(string(data))
	if err != nil {
		return WorkspaceConfig{}, err
	}
	cfg.ResolvePaths(filepath.Dir(path))
	return cfg, nil
}

// ResolvePaths turns relative roots into absolute paths based on `base`.
func (c *WorkspaceConfig) ResolvePaths(base string) {
	for i := range c.Libraries {
		root := c.Libraries[i].Root
		if !filepath.IsAbs(root) {
			root = filepath.Join(base, root)
		}
		c.Libraries[i].Root = filepath.Clean(root)
	}
}

func finalizeLibraryConfig(raw map[string]any) LibraryConfig {
	cfg := LibraryConfig{Extra: map[string]any{}}
	if raw == nil {
		return cfg
	}
	for key, val := range raw {
		switch key {
		case "preview":
			if s, ok := val.(string); ok {
				cfg.Preview = s
				continue
			}
		case "allow_exec":
			if b, ok := val.(bool); ok {
				v := b
				cfg.AllowExec = &v
				continue
			}
		}
		cfg.Extra[key] = val
	}
	return cfg
}

// librarySpecOut is the serialization DTO for one [[libraries]] entry.
type librarySpecOut struct {
	Name   string         `toml:"name"`
	Root   string         `toml:"root"`
	Config map[string]any `toml:"config,omitempty"`
}

// workspaceConfigOut mirrors WorkspaceConfig for stable serialization: known
// fields plus a raw config table merged from known keys and Extra.
type workspaceConfigOut struct {
	Default   string           `toml:"default,omitempty"`
	Libraries []librarySpecOut `toml:"libraries,omitempty"`
}

// MarshalWorkspaceConfig serializes a WorkspaceConfig back to `zoro.toml`
// text. Roots are written exactly as stored (callers that want to preserve the
// original relative form should use ParseWorkspaceConfig, not
// WorkspaceConfigFromPath). Known library-config keys are merged with Extra.
func MarshalWorkspaceConfig(cfg WorkspaceConfig) (string, error) {
	out := workspaceConfigOut{
		Default:   cfg.Default,
		Libraries: make([]librarySpecOut, 0, len(cfg.Libraries)),
	}
	for _, spec := range cfg.Libraries {
		lo := librarySpecOut{Name: spec.Name, Root: spec.Root}
		if m := libraryConfigToMap(spec.Config); m != nil {
			lo.Config = m
		}
		out.Libraries = append(out.Libraries, lo)
	}
	b, err := toml.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WriteWorkspaceConfig serializes and writes a `zoro.toml`, creating the
// parent directory when needed (best effort; empty/relative parents are fine).
func WriteWorkspaceConfig(path string, cfg WorkspaceConfig) error {
	text, err := MarshalWorkspaceConfig(cfg)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func libraryConfigToMap(c LibraryConfig) map[string]any {
	m := map[string]any{}
	if c.Preview != "" {
		m["preview"] = c.Preview
	}
	if c.AllowExec != nil {
		m["allow_exec"] = *c.AllowExec
	}
	for k, v := range c.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}
