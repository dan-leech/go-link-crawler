package config

type CrawlerConfig struct {
	Depth              int  `mapstructure:"depth"`
	Workers            int  `mapstructure:"workers"`
	UseRegexForParsing bool `mapstructure:"use_regex_for_parsing"`
}
