package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	GeneralParams    GeneralParams
	HttpServerParams HttpServerParams
	MainDBParams     MainDBParams
	S3Params         S3Params
}

type GeneralParams struct {
	Env       string
	SecretKey string
}

type HttpServerParams struct {
	Address string
	Port    string
}

type MainDBParams struct {
	Username string
	Password string
	Name     string
	Port     int
	Host     string
	Timeout  int
}

type S3Params struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
}

type ConfigManager struct {
	v      *viper.Viper
	config *Config
}

// NewConfigManager creates new config manager that handles
// all viper config options and loads a config from yaml
func NewConfigManager(configPath string) (*ConfigManager, error) {
	v := viper.New()

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	v.AutomaticEnv()
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	cm := &ConfigManager{v: v}

	if err := cm.loadConfig(); err != nil {
		return nil, err
	}

	return cm, nil
}

// Extracting data from yaml file and loading into Config
func (cm *ConfigManager) loadConfig() error {
	cm.config = &Config{
		GeneralParams: GeneralParams{
			Env:       cm.v.GetString("general_params.env"),
			SecretKey: cm.v.GetString("general_params.secret_key"),
		},
		HttpServerParams: HttpServerParams{
			Address: cm.v.GetString("http_server_params.http_server_address"),
			Port:    cm.v.GetString("http_server_params.http_server_port"),
		},
		MainDBParams: MainDBParams{
			Username: cm.v.GetString("main_db_params.db_username"),
			Password: cm.v.GetString("main_db_params.db_password"),
			Name:     cm.v.GetString("main_db_params.db_name"),
			Port:     cm.v.GetInt("main_db_params.db_port"),
			Host:     cm.v.GetString("main_db_params.db_host"),
			Timeout:  cm.v.GetInt("main_db_params.db_timeout"),
		},
		S3Params: S3Params{
			Endpoint:        cm.v.GetString("s3_params.endpoint"),
			AccessKeyID:     cm.v.GetString("s3_params.access_key_id"),
			SecretAccessKey: cm.v.GetString("s3_params.secret_access_key"),
			UseSSL:          cm.v.GetBool("s3_params.use_ssl"),
			BucketName:      cm.v.GetString("s3_params.bucket_name"),
		},
	}
	return nil
}

// Geting config instance
func (cm *ConfigManager) GetConfig() *Config {
	return cm.config
}

// Compiling a string to connect to main_db
func (db *MainDBParams) GetDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?connect_timeout=%d&sslmode=disable",
		db.Username,
		db.Password,
		db.Host,
		db.Port,
		db.Name,
		db.Timeout,
	)
}

func (h *HttpServerParams) GetAddress() string {
	return fmt.Sprintf(
		"%s:%s",
		h.Address,
		h.Port,
	)
}

func (c *Config) Validate() error {
	// Checking secret key
	if c.GeneralParams.SecretKey == "" {
		return fmt.Errorf("parameter secret_key is required")
	}

	// Checking out enviroment variable
	switch c.GeneralParams.Env {
	case "dev", "prod", "test":
	default:
		return fmt.Errorf("env parameter is invalid: %s. try dev/prod/test instead", c.GeneralParams.Env)
	}

	// Checking http server parameters
	if c.HttpServerParams.Address == "" {
		return fmt.Errorf("%s: http server address is required", c.HttpServerParams.Address)
	}
	if c.HttpServerParams.Port == "" {
		return fmt.Errorf("%s: http server port is required", c.HttpServerParams.Port)
	}

	// Checking MainDbparams
	for name, mainDbConf := range map[string]MainDBParams{
		"MainDB": c.MainDBParams,
	} {
		if mainDbConf.Host == "" {
			return fmt.Errorf("%s: host is required", name)
		}
		if mainDbConf.Username == "" {
			return fmt.Errorf("%s: username is required", name)
		}
		if mainDbConf.Password == "" {
			return fmt.Errorf("%s: password is requred", name)
		}
		if mainDbConf.Port != 5432 {
			return fmt.Errorf("%s: port is invalid", name)
		}
	}

	// Checking S3 params
	if c.S3Params.Endpoint == "" {
		return fmt.Errorf("S3 endpoint is required")
	}
	if c.S3Params.AccessKeyID == "" {
		return fmt.Errorf("S3 access_key id is required")
	}
	if c.S3Params.SecretAccessKey == "" {
		return fmt.Errorf("S3 secret_access_key is required")
	}
	if c.S3Params.BucketName == "" {
		return fmt.Errorf("S3 bucket name is required")
	}

	return nil
}
