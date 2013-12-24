package combainer

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"launchpad.net/goyaml"

	"github.com/cocaine/cocaine-framework-go/cocaine"
	"github.com/noxiouz/Combaine/common"
)

type combainerMainCfg struct {
	Http_hand       string "HTTP_HAND"
	MaxPeriod       uint   "MAXIMUM_PERIOD"
	MaxAttemps      uint   "MAX_ATTEMPS"
	MaxRespWaitTime uint   "MAX_RESP_WAIT_TIME"
	MinimumPeriod   uint   "MINIMUM_PERIOD"
	CloudHosts      string "cloud"
}

type combainerLockserverCfg struct {
	Id      string   "app_id"
	Hosts   []string "host"
	Name    string   "name"
	timeout uint     "timeout"
}

type combainerConfig struct {
	Combainer struct {
		Main          combainerMainCfg       "Main"
		LockServerCfg combainerLockserverCfg "Lockserver"
	} "Combainer"
}

type Client struct {
	Main       combainerMainCfg
	LSCfg      combainerLockserverCfg
	DLS        LockServer
	lockname   string
	cloudHosts []string
	clientStats
}

type clientStats struct {
	sync.RWMutex
	success int
	failed  int
}

type StatInfo struct {
	Success int
	Failed  int
	Total   int
}

func (cs *clientStats) AddSuccess() {
	cs.Lock()
	cs.success++
	cs.Unlock()
}

func (cs *clientStats) AddFailed() {
	cs.Lock()
	cs.failed++
	cs.Unlock()
}

func (cs *clientStats) GetStats() (info *StatInfo) {
	cs.RLock()
	var success = cs.success
	var failed = cs.failed
	cs.RUnlock()
	info = &StatInfo{
		Success: success,
		Failed:  failed,
		Total:   success + failed,
	}
	return
}

// Public API

func NewClient(config string) (*Client, error) {
	// Read combaine.yaml
	data, err := ioutil.ReadFile(config)
	if err != nil {
		return nil, err
	}

	// Parse combaine.yaml
	var m combainerConfig
	err = goyaml.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	// Zookeeper hosts. Connect to Zookeeper
	hosts := m.Combainer.LockServerCfg.Hosts
	dls, err := NewLockServer(strings.Join(hosts, ","))
	if err != nil {
		return nil, err
	}

	cloudHosts, err := GetHosts(m.Combainer.Main.Http_hand, m.Combainer.Main.CloudHosts)
	if err != nil {
		return nil, err
	}
	log.Println(cloudHosts)
	return &Client{
		Main:       m.Combainer.Main,
		LSCfg:      m.Combainer.LockServerCfg,
		DLS:        *dls,
		lockname:   "",
		cloudHosts: cloudHosts,
	}, nil
}

func (cl *Client) Close() {
	cl.DLS.Close()
}

func (cl *Client) Dispatch() {
	defer cl.Close()
	lockpoller := cl.acquireLock()
	if lockpoller != nil {
		log.Println("Acquire Lock", cl.lockname)
	} else {
		return
	}
	_observer.RegisterClient(cl, cl.lockname)
	defer _observer.UnregisterClient(cl.lockname)

	var p_tasks []common.ParsingTask
	var agg_tasks []common.AggregationTask
	res, err := loadConfig(cl.lockname)
	if err != nil {
		log.Println(err)
		return
	}

	var metahost string
	if len(res.Metahost) != 0 {
		metahost = res.Metahost
	} else {
		metahost = res.Groups[0]
	}
	log.Printf("Metahost %s", metahost)
	// Make list of hosts
	var hosts []string
	for _, item := range res.Groups {
		if hosts_for_group, err := GetHosts(cl.Main.Http_hand, item); err != nil {
			log.Println(item, err)
		} else {
			hosts = append(hosts, hosts_for_group...)
		}
		log.Println(hosts)
	}
	// Tasks for parsing
	//host_name, config_name, group_name, previous_time, current_time
	for _, host := range hosts {
		p_tasks = append(p_tasks, common.ParsingTask{
			Host:     host,
			Config:   cl.lockname,
			Group:    res.Groups[0],
			PrevTime: -1,
			CurrTime: -1,
			Id:       "",
			Metahost: metahost,
		})
	}
	//groupname, config_name, agg_config_name, previous_time, current_time
	for _, cfg := range res.AggConfigs {
		agg_tasks = append(agg_tasks, common.AggregationTask{
			Config:   cfg,
			PConfig:  cl.lockname,
			Group:    res.Groups[0],
			PrevTime: -1,
			CurrTime: -1,
			Id:       "",
			Metahost: metahost,
		})
	}

	PARSING_TIME := time.Duration(float64(cl.Main.MinimumPeriod)*0.6) * time.Second
	WHOLE_TIME := time.Duration(cl.Main.MinimumPeriod) * time.Second

	// Dispatch
	var deadline time.Time
	var startTime time.Time
	var wg sync.WaitGroup

	for {
		// Start periodically
		startTime = time.Now()
		deadline = startTime.Add(PARSING_TIME)
		log.Println("Start new iteration at ", startTime)

		for i, task := range p_tasks {
			// Description of task
			task.PrevTime = startTime.Unix()
			task.CurrTime = startTime.Add(WHOLE_TIME).Unix()
			h := md5.New()
			io.WriteString(h, (fmt.Sprintf("%v", task)))
			task.Id = fmt.Sprintf("%x", h.Sum(nil))

			log.Println("Send task number ", i, task)
			go cl.parsingTaskHandler(task, &wg, deadline)
			wg.Add(1)
		}
		wg.Wait()
		log.Println("Parsing finishs ", time.Now())

		deadline = startTime.Add(WHOLE_TIME)
		for i, task := range agg_tasks {
			task.PrevTime = startTime.Unix()
			task.CurrTime = startTime.Add(WHOLE_TIME).Unix()
			h := md5.New()
			io.WriteString(h, (fmt.Sprintf("%v", task)))
			task.Id = fmt.Sprintf("%x", h.Sum(nil))
			log.Println("Send task number ", i, task)
			wg.Add(1)
			go cl.aggregationTaskHandler(task, &wg, deadline)
		}
		wg.Wait()
		log.Println("Aggregation finishs ", time.Now())
		select {
		case <-lockpoller: // Lock
			log.Println("do exit")
			return
		case <-time.After(deadline.Sub(time.Now())):
			log.Println("Go to the next iteration")
		}
	}
}

func (cl *Client) parsingTaskHandler(task common.ParsingTask, wg *sync.WaitGroup, deadline time.Time) {
	defer (*wg).Done()
	limit := deadline.Sub(time.Now())

	var app *cocaine.Service
	var err error
	for deadline.After(time.Now()) {
		host := fmt.Sprintf("%s:10053", cl.getRandomHost())
		app, err = cocaine.NewService(common.PARSING, host)
		if err == nil {
			defer app.Close()
			break
		}
		time.Sleep(200 * time.Microsecond)
	}

	if app == nil {
		log.Printf("Unable to send task %s. Application is unavailable", task.Id)
		cl.clientStats.AddFailed()
		return
	}

	raw, _ := common.Pack(task)
	select {
	case <-time.After(limit):
		log.Printf("Task %s has been late\n", task.Id)
		cl.clientStats.AddFailed()
	case res := <-app.Call("enqueue", "handleTask", raw):
		log.Printf("Receive result %v\n", res.Err())
		cl.clientStats.AddSuccess()
	}
}

func (cl *Client) aggregationTaskHandler(task common.AggregationTask, wg *sync.WaitGroup, deadline time.Time) {
	defer (*wg).Done()
	limit := deadline.Sub(time.Now())

	var app *cocaine.Service
	var err error
	for deadline.After(time.Now()) {
		host := fmt.Sprintf("%s:10053", cl.getRandomHost())
		app, err = cocaine.NewService(common.AGGREGATE, host)
		if err == nil {
			defer app.Close()
			break
		}
		time.Sleep(time.Millisecond * 300)
	}

	if app == nil {
		cl.clientStats.AddFailed()
		log.Printf("Unable to send aggregate task %s. Application is unavailable", task.Id)
		return
	}

	raw, _ := common.Pack(task)
	select {
	case <-time.After(limit):
		log.Println("Task %s has been late", task.Id)
		cl.clientStats.AddFailed()
	case res := <-app.Call("enqueue", "handleTask", raw):
		log.Println("Aggregate done ", res)
		cl.clientStats.AddSuccess()
	}
}

func (cl *Client) getRandomHost() string {
	max := len(cl.cloudHosts)
	return cl.cloudHosts[rand.Intn(max)]
}

// Private API
func (cl *Client) acquireLock() chan bool {
	for _, i := range getParsings() {
		lockname := fmt.Sprintf("/%s/%s", cl.LSCfg.Id, i)
		poller := cl.DLS.AcquireLock(lockname)
		if poller != nil {
			cl.lockname = i
			return poller
		}
	}
	return nil
}
