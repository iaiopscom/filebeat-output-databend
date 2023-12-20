package filebeat_output_databend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"strings"
	"sync"
	"time"

	_ "github.com/datafuselabs/databend-go"
)

type client struct {
	log                   *logp.Logger
	observer              outputs.Observer
	url                   string
	table                 string
	columns               []string
	retryInterval         int
	connect               *sql.DB
	mutex                 sync.Mutex
	skipUnexpectedTypeRow bool
}

func newClient(
	observer outputs.Observer,
	url string,
	table string,
	columns []string,
	retryInterval int,
	skipUnexpectedTypeRow bool,
) *client {
	return &client{
		log:                   logp.NewLogger("databend"),
		observer:              observer,
		url:                   url,
		table:                 table,
		columns:               columns,
		retryInterval:         retryInterval,
		skipUnexpectedTypeRow: skipUnexpectedTypeRow,
	}
}

func (c *client) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.connect != nil {
		c.log.Infof("connection reuse")
		return nil
	}
	c.log.Infof(c.url)
	connect, err := sql.Open("databend", c.url)
	if err != nil {
		c.connect = nil
		c.log.Errorf("open databend connection fail: {%+v}", err)
		return err
	}
	if err = connect.Ping(); err != nil {
		c.connect = nil
		c.log.Errorf("ping databend fail: {%+v}", err)
		return err
	}
	c.log.Infof("new connection")

	c.connect = connect
	return err
}

func (c *client) Close() error {
	c.log.Infof("close connection")
	return c.connect.Close()
}

func (c *client) Publish(_ context.Context, batch publisher.Batch) error {
	if c == nil {
		panic("no client")
	}
	if batch == nil {
		panic("no batch")
	}

	events := batch.Events()
	c.observer.NewBatch(len(events))
	rest, err := c.publish(events)
	if rest != nil {
		c.observer.Failed(len(rest))
		c.sleepBeforeRetry(err)
		batch.RetryEvents(rest)
		return err
	}

	batch.ACK()
	return err
}

func (c *client) String() string {
	return "databend(" + c.url + ")"
}

// publish events
func (c *client) publish(data []publisher.Event) ([]publisher.Event, error) {
	ctx := context.Background()
	formatRows := make([][]interface{}, 0)
	// group events
	okFormatEvents, failFormatEvents, formatRows := extractDataFromEvent(c.log, formatRows, data, c.columns)
	c.log.Infof("[check data format] ok-format-events: %d, fail-format-events: %d, format-rows: %d", len(okFormatEvents), len(failFormatEvents), len(formatRows))

	if len(okFormatEvents) == 0 {
		return failFormatEvents, errors.New("[check data format] all events match field fail")
	}
	tx, err := c.connect.BeginTx(ctx, nil)
	if err != nil {
		c.log.Errorf("[transaction] begin fail: {%+v}", err)
		return data, err
	}
	stmt, err := tx.PrepareContext(ctx, generateSql(c.table, c.columns))
	if err != nil {
		c.log.Errorf("[transaction] stmt prepare fail: {%+v}", err)
		return data, err
	}
	// defer
	defer stmt.Close()
	var lastErr error
	var okExecEvents, failExecEvents []publisher.Event
	for k, row := range formatRows {
		_, err = stmt.ExecContext(ctx, row...)
		if err != nil {
			c.log.Errorf("[transaction] stmt exec fail: {%+v}", err)

			lastErr = err
			// fail
			failExecEvents = append(failExecEvents, okFormatEvents[k])
			continue
		}
		// ok
		okExecEvents = append(okExecEvents, okFormatEvents[k])
	}
	c.log.Infof("[check data type] ok-exec-events: %d, fail-exec-events: %d", len(okExecEvents), len(failExecEvents))
	// happen error, skip unexpected type row
	if lastErr != nil && c.skipUnexpectedTypeRow {
		// rollback
		tx.Rollback()
		if len(okExecEvents) > 0 {
			c.log.Infof("[skip unexpected type row] recall publish, ok-exec-events: %d, fail-exec-events: %d", len(okExecEvents), len(failExecEvents))
			c.publish(okExecEvents)
		}
		return nil, nil
	}
	if err = tx.Commit(); err != nil {
		tx.Rollback()
		c.log.Errorf("[transaction] commit failed, ok-exec-events: %d, fail-exec-events: %d, err: {%+v}", len(okExecEvents), len(failExecEvents), err)
		return data, err
	}

	c.log.Infof("[transaction] commit successed, ok-exec-events: %d, fail-exec-events: %d", len(okExecEvents), len(failExecEvents))
	return failExecEvents, lastErr
}

// sleepBeforeRetry sleep before retry
func (c *client) sleepBeforeRetry(err error) {
	c.log.Errorf("will sleep for %v seconds because an error occurs: %s", c.retryInterval, err)
	time.Sleep(time.Second * time.Duration(c.retryInterval))
}

// generateSql
func generateSql(table string, columns []string) string {
	size := len(columns) - 1
	var columnStr, valueStr strings.Builder
	for i, cl := range columns {
		columnStr.WriteString(cl)
		valueStr.WriteString("?")
		if i < size {
			columnStr.WriteString(",")
			valueStr.WriteString(",")
		}
	}

	return fmt.Sprint("insert into ", table, " (", columnStr.String(), ") values (", valueStr.String(), ")")
}

// extractDataFromEvent extract data
func extractDataFromEvent(
	log *logp.Logger,
	to [][]interface{},
	data []publisher.Event,
	columns []string,
) ([]publisher.Event, []publisher.Event, [][]interface{}) {
	var okEvents, failEvents []publisher.Event
	for _, event := range data {
		content := event.Content
		fmt.Println("content", content)
		row, err := matchFields(content, columns)
		if err != nil {
			log.Errorf("match field error: {%+v}", err)
			// match fail then append fail-events
			failEvents = append(failEvents, event)
			continue
		}
		to = append(to, row)
		// match successed then append ok-events
		okEvents = append(okEvents, event)
	}
	return okEvents, failEvents, to
}

// matchFields match field format
func matchFields(content beat.Event, columns []string) ([]interface{}, error) {
	row := make([]interface{}, 0)
	fmt.Println("content.Fields", content.Fields)
	for _, col := range columns {
		if _, ok := content.Fields[col]; !ok {
			return nil, errors.New("format error")
		}
		val, err := content.GetValue(col)
		if err != nil {
			return nil, err
		}
		// strict mode
		//if val == nil {
		//	return nil, errors.New("row field is empty")
		//}
		row = append(row, val)
	}
	return row, nil
}
