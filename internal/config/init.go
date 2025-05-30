package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/sqweek/dialog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	BuildHash    string // 构建提交Hash
	BuildVersion string // 构建版本
)

var ErrInvalidHostPort = errors.New("invalid port specified in OLLAMA_HOST")

var (
	WorkDir        = "~/.ollama-desktop"
	ConfigFileName = "config/ollama-desktop.json"
	DbFileName     = "config/ollama-desktop.db"
	LogFileName    = "log/ollama-desktop.log"
)

// 日志配置
type Logging struct {
	Level      string `json:"level"`      // 日志级别
	TimeFormat string `json:"timeFormat"` // 时间格式化
	// Filename   string `json:"filename"`   // 日志文件
	MaxSize    int  `json:"maxSize"`    // 文件最大尺寸（以MB为单位）
	MaxBackups int  `json:"maxBackups"` // 保留的最大旧文件数量
	MaxAge     int  `json:"maxAge"`     // 保留旧文件的最大天数
	Compress   bool `json:"compress"`   // 是否压缩/归档旧文件
	LocalTime  bool `json:"localTime"`  // 使用本地时间创建时间戳
}

// 代理配置
type Proxy struct {
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Ollama配置
type OllamaHost struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   string `json:"port"`
}

func (o *OllamaHost) String() string {
	return fmt.Sprintf("%s://%s:%s", nvl(o.Scheme, "http"), nvl(o.Host, "127.0.0.1"), nvl(o.Port, "11434"))
}

type Ollama struct {
	Host *OllamaHost `json:"host"`
}

type AppConfig struct {
	Width                    int  `json:"width"`
	Height                   int  `json:"height"`
	MinWidth                 int  `json:"minWidth"`
	MinHeight                int  `json:"minHeight"`
	AlwaysOnTop              bool `json:"alwaysOnTop"`
	EnableDefaultContextMenu bool `json:"enableDefaultContextMenu"`
	SingleInstance           bool `json:"singleInstance"`

	Logging Logging `json:"logging"`
	Ollama  Ollama  `json:"ollama"`
	Proxy   *Proxy  `json:"proxy"`
}

var Config AppConfig

// 初始化配置
func init() {
	defer func() {
		if r := recover(); r != nil {
			dialog.Message("初始化配置失败(%+v)", r).Title("异常").Error()
			os.Exit(1)
		}
	}()

	initWorkDir()
	initDefaultConfig()
	loadConfigFromFile()
}

func initWorkDir() {
	workDir := os.Getenv("OLLAMA_DESKTOP_WORKDIR")
	workDir = strings.TrimSpace(strings.Trim(strings.TrimSpace(workDir), "\"'"))
	if workDir != "" {
		WorkDir = workDir
	}
	var err error
	WorkDir, err = homedir.Expand(WorkDir)
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(WorkDir, os.ModePerm); err != nil {
		panic(err)
	}
	ConfigFileName = filepath.Join(WorkDir, ConfigFileName)
	DbFileName = filepath.Join(WorkDir, DbFileName)
	LogFileName = filepath.Join(WorkDir, LogFileName)
}

func initDefaultConfig() {
	Config.Width = 1024
	Config.Height = 768
	Config.MinWidth = 1024
	Config.MinHeight = 768
	Config.AlwaysOnTop = false
	Config.EnableDefaultContextMenu = false
	Config.SingleInstance = true

	Config.Logging.Level = "info"
	Config.Logging.TimeFormat = time.DateTime
	// Config.Logging.Filename = "log/ollama-desktop.log"
	Config.Logging.MaxSize = 10
	Config.Logging.MaxBackups = 20
	Config.Logging.MaxAge = 7
	Config.Logging.Compress = true
	Config.Logging.LocalTime = true

	host, err := getOllamaHost()
	if err == nil {
		Config.Ollama.Host = host
	}
}

// 从文件加载配置
func loadConfigFromFile() {
	exist, err := isExist(ConfigFileName)
	if err != nil || !exist {
		// 忽略
		return
	}

	buf, err := os.ReadFile(ConfigFileName)

	_ = json.Unmarshal(buf, &Config)
}

// 判断文件是否存在
func isExist(filename string) (bool, error) {
	if filename == "" {
		return false, nil
	}
	// 使用 os.Stat 判断文件是否存在
	if _, err := os.Stat(filename); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func getOllamaHost() (*OllamaHost, error) {
	defaultPort := "11434"
	hostVar := os.Getenv("OLLAMA_HOST")
	hostVar = strings.TrimSpace(strings.Trim(strings.TrimSpace(hostVar), "\"'"))

	scheme, hostport, ok := strings.Cut(hostVar, "://")
	switch {
	case !ok:
		scheme, hostport = "http", hostVar
	case scheme == "http":
		defaultPort = "80"
	case scheme == "https":
		defaultPort = "443"
	}

	// trim trailing slashes
	hostport = strings.TrimRight(hostport, "/")

	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host, port = "127.0.0.1", defaultPort
		if ip := net.ParseIP(strings.Trim(hostport, "[]")); ip != nil {
			host = ip.String()
		} else if hostport != "" {
			host = hostport
		}
	}

	if portNum, err := strconv.ParseInt(port, 10, 32); err != nil || portNum > 65535 || portNum < 0 {
		return &OllamaHost{
			Scheme: scheme,
			Host:   host,
			Port:   defaultPort,
		}, ErrInvalidHostPort
	}

	return &OllamaHost{
		Scheme: scheme,
		Host:   host,
		Port:   port,
	}, nil
}

func nvl(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
