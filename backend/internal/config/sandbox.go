package config

import (
	"errors"
	"path/filepath"
	"strings"
)

// SandboxConfig exposes application policy, not the runtime's escape hatches.
// It is loaded once at server startup; model settings saves preserve this block.
type SandboxConfig struct {
	ReadRoots []string               `yaml:"read_roots" json:"read_roots"`
	Network   SandboxNetworkConfig   `yaml:"network" json:"network"`
	Execution SandboxExecutionConfig `yaml:"execution" json:"execution"`
}

type SandboxNetworkConfig struct {
	AllowedDomains []string `yaml:"allowed_domains" json:"allowed_domains"`
}

type SandboxExecutionConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
	MaxOutputBytes int `yaml:"max_output_bytes" json:"max_output_bytes"`
}

func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{ReadRoots: []string{}, Network: SandboxNetworkConfig{AllowedDomains: []string{}},
		Execution: SandboxExecutionConfig{TimeoutSeconds: 120, MaxOutputBytes: 8 << 20}}
}

func ValidateSandboxConfig(cfg SandboxConfig) error {
	if cfg.Execution.TimeoutSeconds < 1 || cfg.Execution.TimeoutSeconds > 900 {
		return errors.New("sandbox.execution.timeout_seconds must be between 1 and 900")
	}
	if cfg.Execution.MaxOutputBytes < 1024 || cfg.Execution.MaxOutputBytes > 64<<20 {
		return errors.New("sandbox.execution.max_output_bytes must be between 1024 and 67108864")
	}
	for _, root := range cfg.ReadRoots {
		if !filepath.IsAbs(root) || filepath.Clean(root) == "/" || strings.ContainsRune(root, 0) {
			return errors.New("sandbox.read_roots must contain absolute paths below the filesystem root")
		}
	}
	// Domain syntax and resolved-address enforcement belong to sandbox-runtime.
	for _, domain := range cfg.Network.AllowedDomains {
		if domain == "" || strings.TrimSpace(domain) != domain {
			return errors.New("sandbox.network.allowed_domains must contain nonempty domain names")
		}
	}
	return nil
}
