package config

import (
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go-link-crawler/log"
)

var (
	c *Configuration
)

// Configuration struct
type Configuration struct {
	CrawlerConfig CrawlerConfig `mapstructure:"crawler"`
}

func Init() *Configuration {
	var err error
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName(".go-link-crawler")
	v.AddConfigPath("./config/")
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		err = v.ReadInConfig()
		if err != nil {
			log.WithTrace("config", "Init").Error("error in parsing configuration file:", err)
			return
		}
		err = v.Unmarshal(&c)
	})
	err = v.ReadInConfig()
	if err != nil {
		log.WithTrace("config", "Init").Fatal("error in parsing configuration file:", err)
	}
	err = v.Unmarshal(&c)
	if err != nil {
		log.WithTrace("config", "Init").Fatal("error in unmarshal configuration file:", err)
	}

	return c
}
