package filebeat_output_databend

import (
	"errors"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
)

var logger = logp.NewLogger("Databend")

func init() {
	outputs.RegisterType("databend", makeDatabend)
}

func makeDatabend(
	_ outputs.IndexManager,
	beat beat.Info,
	observer outputs.Observer,
	cfg *common.Config,
) (outputs.Group, error) {

	config := defaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return outputs.Fail(err)
	}

	if len(config.Table) == 0 {
		return outputs.Fail(errors.New("Databend: the table name must be set"))
	}

	if len(config.Columns) == 0 {
		return outputs.Fail(errors.New("Databend: the table columns must be set"))
	}

	client := newClient(observer, config.Url, config.Table, config.Columns, config.RetryInterval, config.SkipUnexpectedTypeRow)

	return outputs.Success(config.BulkMaxSize, config.MaxRetries, client)
}
