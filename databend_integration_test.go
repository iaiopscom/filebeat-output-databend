package filebeat_output_databend

import (
	"database/sql"
	"fmt"
	_ "github.com/datafuselabs/databend-go"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/outest"
	"github.com/gofrs/uuid"
	"math/rand"
	"testing"
	"time"
)

var (
	databendUrl = "tcp://127.0.0.1:9000?debug=true"
	columns     = [3]string{"id", "name", "created_date"}
)

func TestPublish(t *testing.T) {
	databendConfig := map[string]interface{}{
		"url":     databendUrl,
		"table":   "ck_test",
		"columns": columns,
	}

	err := prepare()
	if err != nil {
		t.Fatalf("Error preparing test env: %v", err)
		return
	}

	testPublishList(t, databendConfig)

	err = clean()
	if err != nil {
		t.Fatalf("Error cleaning test env: %v", err)
		return
	}
}

func testPublishList(t *testing.T, cfg map[string]interface{}) {
	batches := 100
	batchSize := 1000

	output := newDatabendTestingOutput(t, cfg)
	err := sendTestEvents(output, batches, batchSize)
	if err != nil {
		t.Fatalf("Error reading config: %v", err)
	}

}

func newDatabendTestingOutput(t *testing.T, cfg map[string]interface{}) outputs.Client {
	config, err := common.NewConfigFrom(cfg)
	if err != nil {
		t.Fatalf("Error reading config: %v", err)
	}

	plugin := outputs.FindFactory("databend")
	if plugin == nil {
		t.Fatalf("clickhouse output module not registered")
	}

	out, err := plugin(beat.Info{Beat: "libbeat"}, outputs.NewNilObserver(), config)
	if err != nil {
		t.Fatalf("Failed to initialize clickhouse output: %v", err)
	}

	client := out.Clients[0].(outputs.NetworkClient)
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to clickhouse host: %v", err)
	}

	return client
}

func sendTestEvents(out outputs.Client, batches, N int) error {
	i := 1
	for b := 0; b < batches; b++ {
		events := make([]beat.Event, N)
		for n := range events {
			events[n] = createEvent(i)
			i++
		}

		batch := outest.NewBatch(events...)
		err := out.Publish(batch)
		if err != nil {
			return err
		}
	}

	return nil
}

func createEvent(message int) beat.Event {
	id, _ := uuid.NewV4()
	return beat.Event{
		Timestamp: time.Now(),
		Meta: common.MapStr{
			"ck-test": "ck-test-MetaValue",
		},
		Fields: common.MapStr{
			"id":           id.String(),
			"name":         fmt.Sprint("ck-test", rand.Intn(100000)),
			"created_date": time.Now(),
		},
	}
}

func prepare() error {
	connect, err := getConn()
	if err != nil {
		return err
	}
	clean()
	_, err = connect.Exec(`
		CREATE TABLE IF NOT EXISTS ck_test (
			id 				FixedString(36),
			name        	FixedString(50),
			created_date    DateTime
		) engine=Memory
	`)

	return err
}

func clean() error {
	connect, err := getConn()
	if err != nil {
		return err
	}
	_, err = connect.Exec(`
		DROP TABLE IF EXISTS ck_test
	`)

	return err
}

func getConn() (*sql.DB, error) {
	connect, err := sql.Open("databend", databendUrl)
	if err != nil {
		return connect, err
	}
	if err := connect.Ping(); err != nil {
		fmt.Println(err)
	}
	return connect, nil
}
