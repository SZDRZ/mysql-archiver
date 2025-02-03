package config

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Datasource struct {
	Addr   string
	User   string
	Pass   string
	Dbname string
}

type TableRule struct {
	Previous *TableRule
	Table    string
	Where    string
	Pk       string
	Key      string
	// Batch_size int
	Deps []TableRule
}

type Config struct {
	Global struct {
		Batch_size int
		Sleep      time.Duration
		Datasource struct {
			Transaction_isolation string
			Src                   Datasource
			Dst                   Datasource
		}
	}
	Rules []TableRule
}

func LoadConfig(cfgPath string) (*Config, error) {

	cfgbuf, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	cfg := new(Config)

	if err := yaml.Unmarshal(cfgbuf, cfg); err != nil {
		return nil, err
	}

	return cfg, nil

}

func ValidConfig(cfg *Config) error {

	var (
		ERR_DS_TRANSACTION_ISOLATION_NOTSUPPORT = errors.New("事务隔离级别不支持")
	)

	/* 校验事务隔离级别设置 */
	switch cfg.Global.Datasource.Transaction_isolation {
	case "READ UNCOMMITTED", "READ COMMITTED", "REPEATABLE READ", "SERIALIZABLE":
		break
	default:
		return ERR_DS_TRANSACTION_ISOLATION_NOTSUPPORT
	}

	return nil

}
