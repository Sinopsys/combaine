package juggler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/combaine/combaine/common"
	"github.com/combaine/combaine/common/httpclient"
	"github.com/combaine/combaine/common/logger"
)

const (
	getChecksURL   = "http://%s/api/checks/checks?%s"
	updateCheckURL = "http://%s/api/checks/add_or_update?do=1"
	sendEventURL   = "http://%s/juggler-fcgi.py?%s"
	defaultTag     = "combaine"
)

type jugglerResponse map[ /*hostname*/ string]map[ /*serviceName*/ string]jugglerCheck

type jugglerChildrenCheck struct {
	Instance string `json:"instance"`
	Host     string `json:"host"`
	Type     string `json:"type"`
	Service  string `json:"service"`
}

type jugglerFlapConfig struct {
	Enable       int64 `codec:"enable" json:"-"`
	BoostTime    int64 `codec:"boost_time" json:"boost_time"`
	StableTime   int64 `codec:"stable_time" json:"stable_time"`
	CriticalTime int64 `codec:"critical_time" json:"critical_time"`
}

type jugglerCheck struct {
	Update           bool                   `json:"-"`
	Host             string                 `json:"host"`
	Service          string                 `json:"service"`
	Description      string                 `json:"description"`
	Aggregator       string                 `json:"aggregator"`
	AggregatorKWArgs aggKWArgs              `json:"aggregator_kwargs"`
	Tags             []string               `json:"tags"`
	Methods          []string               `json:"methods"`
	Children         []jugglerChildrenCheck `json:"children"`
	Flap             *jugglerFlapConfig     `json:"flaps,omitempty"`
}

type aggKWArgs struct {
	IgnoreNoData int                      `codec:"ignore_nodata" json:"ignore_nodata"`
	Limits       []map[string]interface{} `codec:"limits" json:"limits"`
}

type jugglerEvent struct {
	Tags        map[string]string
	Service     string
	Description string
	Level       string
}

// getCheck query juggler api for check
// and Unmarshal json response in to jugglerResponse type
func (js *Sender) getCheck(ctx context.Context) (jugglerResponse, error) {
	var hostChecks jugglerResponse
	var flap map[string]map[string]*jugglerFlapConfig

	var jerrors []error
	if len(js.Tags) == 0 {
		js.Tags = []string{"combaine"}
		logger.Warnf("%s Set query tags to default %s", js.id, js.Tags)
	}
	query := url.Values{
		"do":               {"1"},
		"include_children": {"true"},
		"tag_name":         js.Tags,
	}
	for _, jhost := range js.JHosts {
		//do=1&include_children=true&tag_name=combaine&host_name=
		query.Set("host_name", js.Host)
		url := fmt.Sprintf(getChecksURL, jhost, query.Encode())
		logger.Infof("%s Query check %s", js.id, url)

		resp, err := httpclient.Get(ctx, url)
		switch err {
		case nil:
			body, rerr := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if rerr != nil {
				logger.Errf("%s %s", js.id, rerr)
				jerrors = append(jerrors, fmt.Errorf("failed to read respose from %s: %s", jhost, rerr))
				continue
			}
			logger.Debugf("%s Juggler response %d: %s", js.id, resp.StatusCode, body)

			if resp.StatusCode != http.StatusOK {
				return nil, errors.New(string(body))
			}
			if err := json.Unmarshal(body, &hostChecks); err != nil {
				return nil, fmt.Errorf("Failed to Unmarshal hostChecks: %s", err)
			}
			if err := json.Unmarshal(body, &flap); err != nil {
				return nil, fmt.Errorf("Failed to Unmarshal flaps: %s", err)
			}
			for c, v := range flap[js.Host] {
				if v.StableTime != 0 || v.CriticalTime != 0 || v.BoostTime != 0 {
					chk := hostChecks[js.Host][c]
					chk.Flap = v
					hostChecks[js.Host][c] = chk
				}
			}
			return hostChecks, nil
		case context.Canceled, context.DeadlineExceeded:
			logger.Errf("%s %s", js.id, err)
			return nil, err
		default:
			logger.Errf("%s %s", js.id, err)
			jerrors = append(jerrors, fmt.Errorf("host %s failed with %s", jhost, err))
			continue
		}
	}
	return nil, fmt.Errorf("Failed to get juggler check: %q", jerrors)
}

// ensureCheck check that juggler check exists and it in sync with task data
// if need it call add_or_update check
func (js *Sender) ensureCheck(ctx context.Context, hostChecks jugglerResponse, triggers []jugglerEvent) error {
	services, ok := hostChecks[js.Host]
	if !ok {
		logger.Debugf("%s Create new checks for %s", js.id, js.Host)
		services = make(map[string]jugglerCheck)
		hostChecks[js.Host] = services
	}
	childSet := make(map[string]struct{}) // set
	for serviceName, v := range services {
		for _, c := range v.Children {
			childSet[c.Host+":"+serviceName] = struct{}{}
		}
	}
	for _, t := range triggers {
		check, ok := services[t.Service]
		if !ok {
			logger.Infof("%s Add new check %s.%s", js.id, js.Host, t.Service)
			check = jugglerCheck{Update: true}
		}
		subgroup, err := common.GetSubgroupName(t.Tags)
		if err != nil {
			return err
		}
		t.Tags["name"] = fmt.Sprintf("%s-%s", js.Host, subgroup)

		if t.Tags["type"] == "metahost" {
			logger.Infof("%s Ensure check %s for %s", js.id, t.Service, js.Host)
			// aggregator
			js.ensureAggregator(&check)
			// methods
			js.ensureMethods(&check)
			// flap
			js.ensureFlap(&check)
			// tags
			js.ensureTags(&check)
		}
		// add children
		if _, ok := childSet[t.Tags["name"]+":"+t.Service]; !ok {
			check.Update = true
			child := jugglerChildrenCheck{
				Host:    t.Tags["name"],
				Type:    "HOST", // FIXME? hardcode, delete?
				Service: t.Service,
			}
			logger.Debugf("%s Add children %s", js.id, child)
			check.Children = append(check.Children, child)
		}

		if check.Update {
			check.Host = js.Host
			check.Service = t.Service
			services[t.Service] = check
		}
	}
	for _, c := range services {
		if c.Update {
			if err := js.updateCheck(ctx, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func (js *Sender) ensureAggregator(c *jugglerCheck) {
	aggregatorOutdated := false
	if c.AggregatorKWArgs.IgnoreNoData != js.AggregatorKWArgs.IgnoreNoData {
		aggregatorOutdated = true
	}
	if len(c.AggregatorKWArgs.Limits) != len(js.AggregatorKWArgs.Limits) {
		aggregatorOutdated = true
	}
	if !aggregatorOutdated {
	CHECK:
		for i, v := range js.AggregatorKWArgs.Limits {
			for k, jv := range v {
				if cv, ok := c.AggregatorKWArgs.Limits[i][k]; ok {
					if byteVal, ok := jv.([]byte); ok {
						jv = string(byteVal) // for check below update 'jv'
						v[k] = jv            // json encode []bytes in base64, but there string need
					}
					if fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", jv) {
						aggregatorOutdated = true
						break CHECK
					}
				} else {
					aggregatorOutdated = true
					break CHECK
				}
			}
		}

	}
	if aggregatorOutdated {
		c.Update = true
		logger.Debugf("%s Check outdated, aggregator differ: %s != %s", js.id, c.AggregatorKWArgs, js.AggregatorKWArgs)
		c.Aggregator = js.Aggregator
		c.AggregatorKWArgs = js.AggregatorKWArgs
	}
}

func (js *Sender) ensureMethods(c *jugglerCheck) {
	if len(js.Methods) == 0 {
		if js.Method != "" {
			js.Methods = strings.Split(js.Method, ",")
		} else {
			js.Methods = []string{"GOLEM"}
		}
	}
	methodsOutdated := false
	if len(c.Methods) != len(js.Methods) {
		methodsOutdated = true
	} else {
		checkMSet := make(map[string]struct{}, len(c.Methods))
		for _, m := range c.Methods {
			checkMSet[m] = struct{}{}
		}
		for _, m := range js.Methods {
			if _, ok := checkMSet[m]; !ok {
				methodsOutdated = true
				break
			}
		}
	}
	if methodsOutdated {
		c.Update = true
		logger.Debugf("%s Check outdated, METHODS differ: %s != %s", js.id, c.Methods, js.Methods)
		c.Methods = js.Methods
	}
}

func (js *Sender) ensureFlap(c *jugglerCheck) {
	if c.Flap == nil {
		c.Flap = &jugglerFlapConfig{}
	}
	c.Flap.Enable = 1 // enable field not set by json, set in manually

	if f, ok := js.ChecksOptions[c.Service]; ok {
		if f.Enable == 1 {
			if *c.Flap != f {
				c.Update = true
				logger.Debugf("%s Check outdated, Flap differ: %s != %s", js.id, c.Flap, js.Flap)
				c.Flap = &f
			}
		}
	} else {
		// if flap setting not set for check individually, try apply global settings
		if js.Flap != nil && js.Flap.Enable == 1 {
			if *c.Flap != *js.Flap {
				c.Update = true
				logger.Debugf("%s Check outdated, Flap differ: %s != %s", js.id, c.Flap, js.Flap)
				c.Flap = js.Flap
			}
		} else {
			c.Flap = nil
		}
	}
}

func (js *Sender) ensureTags(c *jugglerCheck) {
	if len(js.Tags) == 0 {
		js.Tags = []string{defaultTag}
	}
	// TODO: tags by servces in juggler config as for flaps?
	if c.Tags == nil || len(c.Tags) == 0 {
		c.Update = true
		logger.Debugf("%s Check outdated, Tags differ: %s != %s", js.id, c.Tags, js.Tags)
		c.Tags = make([]string, len(js.Tags))
		copy(c.Tags, js.Tags)
	} else {
		tagsSet := make(map[string]struct{}, len(c.Tags))
		for _, tag := range c.Tags {
			tagsSet[tag] = struct{}{}
		}
		for _, tag := range js.Tags {
			if _, ok := tagsSet[tag]; !ok {
				c.Update = true
				logger.Infof("%s Add tag %s", js.id, tag)
				c.Tags = append(c.Tags, tag)
			}
		}
	}
}

func (js *Sender) updateCheck(ctx context.Context, check jugglerCheck) error {
	logger.Infof("%s Update check %s for %s", js.id, check.Service, check.Host)

	cJSON, err := json.Marshal(check)
	if err != nil {
		return err
	}
	logger.Debugf("%s JSON payload for updating check: %s", js.id, cJSON)

	errs := make(map[string]string, 0)
	for _, host := range js.JHosts {
		url := fmt.Sprintf(updateCheckURL, host)
		resp, err := httpclient.Post(ctx, url, "application/json", bytes.NewReader(cJSON))
		switch err {
		case nil:
			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				logger.Errf("%s %s", js.id, err)
				errs[err.Error()] = ""
				continue
			}

			if resp.StatusCode != http.StatusOK {
				logger.Warnf("%s Update check query %s: %d - %s", js.id, url, resp.StatusCode, body)
				errs[string(body)] = ""
				continue
			}
			logger.Infof("%s Sucessfully send update %s.%s %s: %s", js.id, check.Host, check.Service, url, body)
			return nil
		case context.Canceled, context.DeadlineExceeded:
			logger.Errf("%s %s", js.id, err)
			return err
		default:
			logger.Errf("%s %s", js.id, err)
			errs[err.Error()] = ""
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", errs)
	}
	logger.Errf("%s failed to sent update check for %v", js.id, check)
	return fmt.Errorf("Upexpected error, can't update check for %s.%s", check.Host, check.Service)
}

// sendEvent send juggler event borned by ensureCheck to juggler's
func (js *Sender) sendEvent(ctx context.Context, front string, event jugglerEvent) error {
	query := url.Values{
		"status":      {event.Level},
		"description": {event.Description},
		"service":     {event.Service},
		"host":        {event.Tags["name"]},
		"instance":    {""},
	}

	url := fmt.Sprintf(sendEventURL, front, query.Encode())
	logger.Debugf("%s Send event %s", js.id, url)
	resp, err := httpclient.Get(ctx, url)
	if err != nil {
		logger.Errf("%s %s", js.id, err)
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		logger.Errf("%s %s", js.id, err)
		return err
	}
	logger.Infof("%s Response %s: %d - %q", js.id, url, resp.StatusCode, body)

	if resp.StatusCode != http.StatusOK {
		return errors.New(string(body))
	}
	return nil
}