package juggler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/combaine/combaine/senders"
	"github.com/combaine/combaine/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	globalTestTask *senders.SenderTask // loaded in TestMain
	ts             *httptest.Server
	payloadFile    = "testdata/payload.yaml"
)

func DefaultJugglerTestConfig() *Config {
	conf := DefaultConfig()
	// add test conditions
	conf.OK = []string{"${nginx}.get('5xx', 0)<0.060"}
	conf.INFO = []string{"${nginx}.get('5xx', 0)<0.260"}
	//conf.WARN = []string{"${nginx}.get('5xx', 0)<0.460"}
	conf.CRIT = []string{"${nginx}.get('5xx', 0)>1.060"}
	// add test config
	conf.JPluginConfig = map[string]interface{}{
		"checks": map[string]interface{}{
			"testTimings": map[string]interface{}{
				"query":  "%S+/%S+/%S+timings/7",
				"status": "WARN",
				"limit":  0.900, // second
			},
			"test4xx": map[string]interface{}{
				"query":  "%S+/%S+/4xx",
				"status": "CRIT",
				"limit":  30,
			},
		},
	}

	testConf := "testdata/config/juggler_example.yaml"
	os.Setenv("JUGGLER_CONFIG", testConf)
	sConf, err := GetSenderConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to load juggler sender config %s: %s", testConf, err))
	}

	conf.PluginsDir = sConf.PluginsDir
	conf.JHosts = sConf.Hosts
	conf.JFrontend = sConf.Frontend

	return conf
}

func TestUpdateTaskConfig(t *testing.T) {
	// Errors when config not exists
	os.Setenv("JUGGLER_CONFIG", "")
	_, err := GetSenderConfig()
	assert.Error(t, err)
	testConf := "testdata/config/nonExistingJugglerConfig.yaml"
	os.Setenv("JUGGLER_CONFIG", testConf)
	_, err = GetSenderConfig()
	assert.Error(t, err)

	// config exists and tests for config parsing
	testConf = "testdata/config/juggler_example.yaml"
	os.Setenv("JUGGLER_CONFIG", testConf)
	conf, err := GetSenderConfig()
	assert.NoError(t, err)
	assert.Equal(t, conf.Hosts[0], "host1")

	testConf = "testdata/config/without_fronts.yaml"
	os.Setenv("JUGGLER_CONFIG", testConf)
	conf, err = GetSenderConfig()
	assert.NoError(t, err)
	assert.Equal(t, conf.PluginsDir, defaultPluginsDir)
	var taskConf Config
	assert.NoError(t, UpdateTaskConfig(&taskConf, conf))
	assert.NotNil(t, taskConf.JFrontend)
	assert.Equal(t, taskConf.JFrontend, taskConf.JHosts)

}

func TestLoadPlugin(t *testing.T) {
	if _, err := LoadPlugin("Test Id", ".", "file_not_exists.lua"); err == nil {
		t.Fatalf("Loading non existing plugin should return error")
	}
	if _, err := LoadPlugin("Test Id", "testdata/plugins", "test"); err != nil {
		t.Fatalf("Failed to load plugin 'test': %s", err)
	}
}

func TestPrepareLuaEnv(t *testing.T) {
	jconf := DefaultJugglerTestConfig()
	jconf.Plugin = "test"

	l, err := LoadPlugin("Test Id", jconf.PluginsDir, jconf.Plugin)
	assert.NoError(t, err)
	js, err := NewSender(jconf, "Test ID")
	assert.NoError(t, err)

	js.state = l
	js.preparePluginEnv(globalTestTask)

	l.Push(l.GetGlobal("testEnv"))
	l.Call(0, 1)

	result := l.ToString(1)
	assert.Equal(t, "OK", result)
}

func TestRunPlugin(t *testing.T) {
	jconf := DefaultJugglerTestConfig()

	js, err := NewSender(jconf, "Test ID")
	assert.NoError(t, err)

	jconf.Plugin = "correct"
	l, err := LoadPlugin("Test Id", jconf.PluginsDir, jconf.Plugin)
	assert.NoError(t, err)
	js.state = l
	t.Logf("Task: %#+v", globalTestTask)
	for idx, res := range globalTestTask.Data {
		t.Logf("Task.Data[%d].Result: %#+v", idx, res.Result)
	}
	assert.NoError(t, js.preparePluginEnv(globalTestTask))

	_, err = js.runPlugin()
	assert.NoError(t, err)

	jconf.Plugin = "incorrect"
	l, err = LoadPlugin("Test Id", jconf.PluginsDir, jconf.Plugin)
	assert.NoError(t, err)
	js.state = l
	assert.NoError(t, js.preparePluginEnv(globalTestTask))
	_, err = js.runPlugin()
	if err == nil {
		err = errors.New("incorrect should fail")
	}
	assert.Contains(t, err.Error(), "Expected 'run' function inside plugin")
}

func TestQueryLuaTable(t *testing.T) {
	jconf := DefaultJugglerTestConfig()

	js, err := NewSender(jconf, "Test ID")
	assert.NoError(t, err)

	jconf.Plugin = "test"
	l, err := LoadPlugin("Test Id", jconf.PluginsDir, jconf.Plugin)
	assert.NoError(t, err)
	js.state = l
	assert.NoError(t, js.preparePluginEnv(globalTestTask))
	l.Push(l.GetGlobal("testQuery"))
	l.Call(0, 1)
	result := l.ToTable(1)

	events, err := js.luaResultToJugglerEvents(result)
	t.Logf("Test events: %v", events)
	assert.NoError(t, err)
	assert.Len(t, events, 32)
}

func TestMain(m *testing.M) {
	dataYaml, yerr := ioutil.ReadFile(payloadFile)
	if yerr != nil {
		panic(yerr)
	}
	var payload []senders.Payload
	if yerr := yaml.Unmarshal(dataYaml, &payload); yerr != nil {
		log.Fatalf("Failed to unmarshal %s: %v", payloadFile, yerr)
		panic(yerr)
	}
	tmpSenderRequest := senders.SenderRequest{Data: make([]*senders.AggregationResult, len(payload))}
	for idx, item := range payload {
		res, err := utils.Pack(item.Result)
		if err != nil {
			log.Fatalf("Failed to pack %s: %v", payloadFile, err)
		}
		tmpSenderRequest.Data[idx] = &senders.AggregationResult{Tags: item.Tags, Result: res}
	}
	globalTestTask, yerr = senders.RepackSenderRequest(&tmpSenderRequest)
	if yerr != nil {
		log.Fatalf("Failed to repack sender request: %s", yerr)
	}

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/checks/checks":
			hostName := r.URL.Query().Get("host_name")
			if hostName == "" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintln(w, "Query parameter host_name not specified")
				return
			}
			fileName := fmt.Sprintf("testdata/checks/%s.json", hostName)
			logrus.Infof("Read check from file %s", fileName)
			resp, err := ioutil.ReadFile(fileName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Failed to read file %s, %s", fileName, err)
				return
			}
			var tmpObject interface{}
			if jsonUnmarshalErr := json.Unmarshal(resp, &tmpObject); jsonUnmarshalErr != nil {
				panic(jsonUnmarshalErr)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tmpObject)
		case "/api/checks/add_or_update":
			reqBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(500)
				fmt.Fprintln(w, err)
				return
			}
			w.WriteHeader(200)
			io.Copy(w, bytes.NewReader(reqBytes))
		case "/juggler-fcgi.py":
			fmt.Fprintln(w, "OK")
		case "/events":
			reqBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(500)
				fmt.Fprintln(w, err)
				return
			}
			w.WriteHeader(200)
			var batch jugglerBatchRequest
			err = json.Unmarshal(reqBytes, &batch)
			if err != nil || batch.Events == nil {
				w.WriteHeader(500)
				fmt.Fprintln(w, errors.Errorf("Unmarshal batch failed %s", err))
				return
			}
			var jResp jugglerBatchResponse
			for _, e := range batch.Events {
				code := 200
				if strings.Contains(e.Host, "nonExisting") {
					code = 400
				}
				jResp.Events = append(jResp.Events, jugglerBatchEventReport{
					Message: "error for nonExisting, just test case with error",
					Code:    code,
				})
			}
			json.NewEncoder(w).Encode(jResp)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "Not Found")
		}
	}))
	defer ts.Close()

	os.Exit(m.Run())
}
