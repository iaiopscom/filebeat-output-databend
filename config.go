package filebeat_output_databend

import (
	"github.com/elastic/beats/v7/libbeat/outputs/codec"
)

type dataBendConfig struct {
	Url                   string       `config:"url"`
	Table                 string       `config:"table"`
	Columns               []string     `config:"columns"`
	Codec                 codec.Config `config:"codec"`
	BulkMaxSize           int          `config:"bulk_max_size"`
	MaxRetries            int          `config:"max_retries"`
	RetryInterval         int          `config:"retry_interval"`
	SkipUnexpectedTypeRow bool         `config:"skip_unexpected_type_row"`
}

var (
	defaultConfig = dataBendConfig{
		Url:                   "http://:@127.0.0.1:9000/default",
		BulkMaxSize:           1000,
		MaxRetries:            3,
		RetryInterval:         60,
		SkipUnexpectedTypeRow: false,
	}
)
