// Package config handles loading configuration from a TOML file and CLI flags.
// It imports no other internal packages.
//
// Loading order:
//  1. Built-in defaults
//  2. Config file (~/.binge-os-watch/server.toml or --config path)
//
// On first run, the base directory and a commented default config are created.
package config

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	AppName    = "binge-os-watch"
	BaseDir    = "." + AppName
	ConfigFile = "server.toml"
)

type Config struct {
	Server  ServerConfig  `toml:"server"`
	DB      DBConfig      `toml:"database"`
	Session SessionConfig `toml:"session"`
	Argon2  Argon2Config  `toml:"argon2"`
	TMDB    TMDBConfig    `toml:"tmdb"`
}

type ServerConfig struct {
	Addr                string `toml:"addr"`
	BaseURL             string `toml:"base_url"`
	DisableUI           bool   `toml:"disable_ui"`
	DisableAPI          bool   `toml:"disable_api"`
	DisableRegistration bool   `toml:"disable_registration"`
	ImageCacheDir       string `toml:"image_cache_dir"`
}

type DBConfig struct {
	Path string `toml:"path"`
}

type SessionConfig struct {
	DurationHours int  `toml:"duration_hours"`
	SecureCookie  bool `toml:"secure_cookie"`
}

type Argon2Config struct {
	Time      uint32 `toml:"time"`
	Memory    uint32 `toml:"memory"`
	Threads   uint8  `toml:"threads"`
	KeyLength uint32 `toml:"key_length"`
	SaltLen   uint32 `toml:"salt_length"`
}

type TMDBConfig struct {
	APIKey               string `toml:"api_key"`
	DefaultRegion        string `toml:"default_region"`
	MetadataSyncInterval string `toml:"metadata_sync_interval"`
	KeywordScanInterval  string `toml:"keyword_scan_interval"`
}

func (c *Config) SessionDuration() time.Duration {
	return time.Duration(c.Session.DurationHours) * time.Hour
}

func (c *Config) MetadataSyncDuration() time.Duration {
	d, err := time.ParseDuration(c.TMDB.MetadataSyncInterval)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

func (c *Config) KeywordScanDuration() time.Duration {
	d, err := time.ParseDuration(c.TMDB.KeywordScanInterval)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

func BasePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, BaseDir)
}

func DefaultConfigPath() string {
	return filepath.Join(BasePath(), ConfigFile)
}

func DefaultDBPath() string {
	return filepath.Join(BasePath(), "data", AppName+".db")
}

func Defaults() Config {
	return Config{
		Server:  ServerConfig{Addr: ":8080", ImageCacheDir: filepath.Join(BasePath(), "cache", "images")},
		DB:      DBConfig{Path: DefaultDBPath()},
		Session: SessionConfig{DurationHours: 720, SecureCookie: false},
		Argon2:  Argon2Config{Time: 1, Memory: 65536, Threads: 4, KeyLength: 32, SaltLen: 16},
		TMDB: TMDBConfig{
			APIKey:               "",
			DefaultRegion:        "NL",
			MetadataSyncInterval: "24h",
			KeywordScanInterval:  "24h",
		},
	}
}

// Load reads configuration: defaults → config file.
// Parses --config from CLI flags. Creates a default config on first run.
func Load() Config {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to config file (default: ~/.binge-os-watch/server.toml)")
	flag.Parse()

	cfg := Defaults()

	if configPath == "" {
		configPath = DefaultConfigPath()
	}

	ensureDir(BasePath())
	ensureDir(filepath.Dir(cfg.DB.Path))

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		slog.Info("config file not found, creating default", "path", configPath)
		if err := writeDefaultConfig(configPath); err != nil {
			slog.Warn("could not write default config", "error", err)
		}
	} else if err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			slog.Error("error reading config", "path", configPath, "error", err)
			os.Exit(1)
		}
		if err := decodeTOML(string(data), &cfg); err != nil {
			slog.Error("error parsing config", "path", configPath, "error", err)
			os.Exit(1)
		}
		slog.Info("loaded config", "path", configPath)
	}

	cfg.DB.Path = expandHome(cfg.DB.Path)
	ensureDir(filepath.Dir(cfg.DB.Path))

	return cfg
}

func decodeTOML(data string, cfg *Config) error {
	_, err := toml.Decode(data, cfg)
	return err
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0750); err != nil {
		slog.Error("cannot create directory", "path", path, "error", err)
		os.Exit(1)
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func writeDefaultConfig(path string) error {
	ensureDir(filepath.Dir(path))
	return os.WriteFile(path, []byte(defaultConfigContent), 0640)
}

const defaultConfigContent = `# binge-os-watch server configuration
# All paths support ~ for the home directory.

[server]
# Address to listen on.
addr = ":8080"

# Set to true to disable the web UI and only serve the API.
# disable_ui = false
# disable_api = false

# Set to true to disable user registration (existing accounts still work).
# disable_registration = false

[database]
# Path to the SQLite database file.
path = "~/.binge-os-watch/data/binge-os-watch.db"

[session]
# Session cookie duration in hours (default: 720 = 30 days).
duration_hours = 720

# Set to true in production behind HTTPS.
secure_cookie = false

[argon2]
# Argon2id password hashing parameters.
# Only change these if you know what you're doing.
time = 1
memory = 65536       # 64 MB
threads = 4
key_length = 32
salt_length = 16

[tmdb]
# TMDB API v3 key (required). Get one at https://www.themoviedb.org/settings/api
api_key = ""

# Default region for watch provider lookups (ISO 3166-1, e.g. "NL", "US").
default_region = "NL"

# How often to refresh TV show metadata (Go duration, e.g. "24h", "12h").
metadata_sync_interval = "24h"

# How often to run keyword watch scans (Go duration).
keyword_scan_interval = "24h"
`
