package combainer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/combaine/combaine/common"
	"github.com/combaine/combaine/common/configs"
	"github.com/combaine/combaine/common/hosts"
	"github.com/combaine/combaine/common/tasks"
)

type sessionParams struct {
	ParallelParsings int
	ParsingTime      time.Duration
	WholeTime        time.Duration
	PTasks           []tasks.ParsingTask
	AggTasks         []tasks.AggregationTask
}

// Client is a distributor of tasks across the computation grid
type Client struct {
	ID         string
	Repository configs.Repository
	*Context
	Log *logrus.Entry
	clientStats
}

// NewClient returns new client
func NewClient(context *Context, repo configs.Repository) (*Client, error) {
	if context.Hosts == nil {
		err := fmt.Errorf("Hosts delegate must be specified")
		context.Logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Unable to create Client")
		return nil, err
	}

	id := GenerateSessionId()
	cl := &Client{
		ID:         id,
		Repository: repo,
		Context:    context,
		Log:        context.Logger.WithField("client", id),
	}
	return cl, nil
}

func (cl *Client) updateSessionParams(config string) (sp *sessionParams, err error) {
	cl.Log.WithFields(logrus.Fields{
		"config": config,
	}).Info("updating session parametrs")

	var (
		// tasks
		pTasks   []tasks.ParsingTask
		aggTasks []tasks.AggregationTask

		// timeouts
		parsingTime time.Duration
		wholeTime   time.Duration
	)

	encodedParsingConfig, err := cl.Repository.GetParsingConfig(config)
	if err != nil {
		cl.Log.WithFields(logrus.Fields{"config": config, "error": err}).Error("unable to load config")
		return nil, err
	}

	var parsingConfig configs.ParsingConfig
	if err = encodedParsingConfig.Decode(&parsingConfig); err != nil {
		cl.Log.WithFields(logrus.Fields{"config": config, "error": err}).Error("unable to decode parsingConfig")
		return nil, err
	}

	cfg := cl.Repository.GetCombainerConfig()
	parsingConfig.UpdateByCombainerConfig(&cfg)
	aggregationConfigs, err := GetAggregationConfigs(cl.Repository, &parsingConfig)
	if err != nil {
		cl.Log.WithFields(logrus.Fields{"config": config, "error": err}).Error("unable to read aggregation configs")
		return nil, err
	}

	cl.Log.Infof("updating config: group %s, metahost %s",
		parsingConfig.GetGroup(), parsingConfig.GetMetahost())

	hostFetcher, err := LoadHostFetcher(cl.Context, parsingConfig.HostFetcher)
	if err != nil {
		cl.Log.WithFields(logrus.Fields{"config": config, "error": err}).Error("Unable to construct SimpleFetcher")
		return
	}

	allHosts := make(hosts.Hosts)
	for _, item := range parsingConfig.Groups {
		hostsForGroup, err := hostFetcher.Fetch(item)
		if err != nil {
			cl.Log.WithFields(logrus.Fields{"config": config, "error": err, "group": item}).Warn("unable to get hosts")
			continue
		}

		allHosts.Merge(&hostsForGroup)
	}

	listOfHosts := allHosts.AllHosts()

	if len(listOfHosts) == 0 {
		err := fmt.Errorf("No hosts in given groups")
		cl.Log.WithFields(logrus.Fields{"config": config, "group": parsingConfig.Groups}).Warn("no hosts in given groups")
		return nil, err
	}

	cl.Log.WithFields(logrus.Fields{"config": config}).Infof("hosts: %s", listOfHosts)

	parallelParsings := len(listOfHosts)
	if parsingConfig.ParallelParsings > 0 && parallelParsings > parsingConfig.ParallelParsings {
		parallelParsings = parsingConfig.ParallelParsings
	}

	// Tasks for parsing
	for _, host := range listOfHosts {
		pTasks = append(pTasks, tasks.ParsingTask{
			CommonTask:         tasks.EmptyCommonTask,
			Host:               host,
			ParsingConfigName:  config,
			ParsingConfig:      parsingConfig,
			AggregationConfigs: *aggregationConfigs,
		})
	}

	for _, name := range parsingConfig.AggConfigs {
		aggTasks = append(aggTasks, tasks.AggregationTask{
			CommonTask:        tasks.EmptyCommonTask,
			Config:            name,
			ParsingConfigName: config,
			ParsingConfig:     parsingConfig,
			AggregationConfig: (*aggregationConfigs)[name],
			Hosts:             allHosts,
		})
	}

	parsingTime, wholeTime = GenerateSessionTimeFrame(parsingConfig.IterationDuration)

	sp = &sessionParams{
		ParallelParsings: parallelParsings,
		ParsingTime:      parsingTime,
		WholeTime:        wholeTime,
		PTasks:           pTasks,
		AggTasks:         aggTasks,
	}

	cl.Log.Info("Session parametrs have been updated successfully")
	cl.Log.WithFields(logrus.Fields{"config": config}).Debugf("Current session parametrs. %v", sp)

	return sp, nil
}

// Dispatch does one iteration of tasks dispatching
func (cl *Client) Dispatch(parsingConfigName string, uniqueID string, shouldWait bool) error {
	GlobalObserver.RegisterClient(cl, parsingConfigName)
	defer GlobalObserver.UnregisterClient(parsingConfigName)

	if uniqueID == "" {
		uniqueID = GenerateSessionId()
	}

	contextFields := logrus.Fields{
		"session": uniqueID,
		"config":  parsingConfigName}

	sessionParameters, err := cl.updateSessionParams(parsingConfigName)
	if err != nil {
		cl.Log.WithFields(logrus.Fields{"session": uniqueID, "config": parsingConfigName, "error": err}).Error("unable to update session parametrs")
		return err
	}

	startTime := time.Now()
	// Context for the whole dispath.
	// It includes parsing, aggregation and wait stages
	wctx, cancelFunc := context.WithDeadline(context.TODO(), startTime.Add(sessionParameters.WholeTime))
	defer cancelFunc()

	cl.Log.WithFields(contextFields).Info("Start new iteration")
	hosts, err := cl.Context.Hosts()
	if err != nil || len(hosts) == 0 {
		cl.Log.WithFields(logrus.Fields{"session": uniqueID, "config": parsingConfigName, "error": err}).Error("unable to get (or empty) the list of the cloud hosts")
		return err
	}

	// Parsing phase
	totalTasksAmount := len(sessionParameters.PTasks)
	tokens := make(chan struct{}, sessionParameters.ParallelParsings)
	parsingResult := make(tasks.ParsingResult)
	var mu sync.Mutex
	pctx, cancelFunc := context.WithDeadline(wctx, startTime.Add(sessionParameters.ParsingTime))
	defer cancelFunc()

	var wg sync.WaitGroup
	for i, task := range sessionParameters.PTasks {
		// Description of task
		task.PrevTime = startTime.Unix()
		task.CurrTime = startTime.Add(sessionParameters.WholeTime).Unix()
		task.CommonTask.Id = uniqueID

		cl.Log.WithFields(contextFields).Infof("Send task number %d/%d to parsing", i+1, totalTasksAmount)
		cl.Log.WithFields(contextFields).Debugf("Parsing task content %s", task)

		wg.Add(1)
		tokens <- struct{}{} // acqure
		go func(t tasks.ParsingTask) {
			defer wg.Done()
			defer func() { <-tokens }() // release
			cl.doParsingTask(pctx, t, &mu, hosts, parsingResult)
		}(task)
	}
	wg.Wait()

	cl.Log.WithFields(contextFields).Infof("Parsing finished for %d hosts", len(parsingResult))
	// Aggregation phase
	totalTasksAmount = len(sessionParameters.AggTasks)
	for i, task := range sessionParameters.AggTasks {
		task.PrevTime = startTime.Unix()
		task.CurrTime = startTime.Add(sessionParameters.WholeTime).Unix()
		task.CommonTask.Id = uniqueID
		task.ParsingResult = parsingResult

		cl.Log.WithFields(contextFields).Infof("Send task number %d/%d to aggregate", i+1, totalTasksAmount)
		cl.Log.WithFields(contextFields).Debugf("Aggregate task content %s", task)
		wg.Add(1)
		go func(t tasks.AggregationTask) {
			defer wg.Done()
			cl.doAggregationHandler(wctx, t, hosts)
		}(task)
	}
	wg.Wait()

	cl.Log.WithFields(contextFields).Info("Aggregation has finished")

	// Wait for next iteration if needed.
	// wctx has a deadline
	if shouldWait {
		<-wctx.Done()
	}

	cl.Log.WithFields(contextFields).Debug("Go to the next iteration")

	return nil
}

func (cl *Client) doGeneralTask(ctx context.Context, appName string, task tasks.Task, hosts []string) (interface{}, error) {
	worker, err := cl.Resolver.Resolve(ctx, appName, hosts)
	if err != nil {
		cl.Log.WithFields(logrus.Fields{"session": task.Tid(), "error": err, "appname": appName}).Error("unable to send task")
		return nil, err
	}

	raw, err := task.Raw()
	if err != nil {
		cl.Log.WithFields(logrus.Fields{"session": task.Tid(), "error": err,
			"appname": appName, "host": worker.Footprint()}).Errorf("failed to unpack task's data for group %s", task.Group())
		return nil, err
	}

	var res interface{}
	if err = worker.Do(ctx, "handleTask", raw).Wait(ctx, &res); err != nil {
		cl.Log.WithFields(logrus.Fields{"session": task.Tid(), "error": err,
			"appname": appName, "host": worker.Footprint()}).Errorf("task for group %s failed", task.Group())
		return nil, err
	}

	cl.Log.WithFields(logrus.Fields{"session": task.Tid(), "appname": appName,
		"host": worker.Footprint()}).Infof("task for group %s done", task.Group())
	return res, nil
}

func (cl *Client) doParsingTask(ctx context.Context, task tasks.ParsingTask, m *sync.Mutex, hosts []string, r tasks.ParsingResult) {
	i, err := cl.doGeneralTask(ctx, common.PARSING, &task, hosts)
	if err != nil {
		cl.clientStats.AddFailedParsing()
		return
	}
	var res tasks.ParsingResult
	if err := common.Unpack(i.([]byte), &res); err != nil {
		cl.clientStats.AddFailedParsing()
		return
	}
	m.Lock()
	for k, v := range res {
		r[k] = v
	}
	m.Unlock()
	cl.clientStats.AddSuccessParsing()

}

func (cl *Client) doAggregationHandler(ctx context.Context, task tasks.AggregationTask, hosts []string) {
	_, err := cl.doGeneralTask(ctx, common.AGGREGATE, &task, hosts)
	if err != nil {
		cl.clientStats.AddFailedAggregate()
		return
	}
	cl.clientStats.AddSuccessAggregate()
}
