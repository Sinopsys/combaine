package main

import (
	"context"
	"flag"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/grpclog"

	"github.com/combaine/combaine/common/logger"
	"github.com/combaine/combaine/rpc"
	"github.com/combaine/combaine/worker"
	"github.com/sirupsen/logrus"

	//_ "net/http/pprof"

	//_ "golang.org/x/net/trace"

	_ "github.com/combaine/combaine/fetchers"
)

var (
	endpoint  string
	logoutput string
	tracing   bool
	loglevel  = logger.LogrusLevelFlag(logrus.InfoLevel)
)

func init() {
	flag.StringVar(&endpoint, "endpoint", ":10052", "endpoint")
	flag.StringVar(&logoutput, "logoutput", "/dev/stderr", "path to logfile")
	flag.BoolVar(&tracing, "trace", false, "enable tracing")
	flag.Var(&loglevel, "loglevel", "debug|info|warn|warning|error|panic in any case")
	flag.Parse()
	grpc.EnableTracing = tracing

	logger.InitializeLogger(loglevel.ToLogrusLevel(), logoutput)
	grpclog.SetLoggerV2(logger.NewLoggerV2WithVerbosity(0))
	worker.InitializeServiceCacher()
}

type server struct{}

func (s *server) DoParsing(ctx context.Context, task *rpc.ParsingTask) (*rpc.ParsingResult, error) {
	return worker.DoParsing(ctx, task)
}

func (s *server) DoAggregating(ctx context.Context, task *rpc.AggregatingTask) (*rpc.AggregatingResult, error) {
	if err := worker.DoAggregating(ctx, task); err != nil {
		return nil, err
	}
	return new(rpc.AggregatingResult), nil
}

func main() {
	//go func() { log.Println(http.ListenAndServe("[::]:8002", nil)) }()

	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(
		grpc.MaxRecvMsgSize(1024*1024*128 /* 128 MB */),
		grpc.MaxSendMsgSize(1024*1024*128 /* 128 MB */),
		grpc.MaxConcurrentStreams(2000),
		grpc.ConnectionTimeout(5*time.Second),
	)
	rpc.RegisterWorkerServer(s, &server{})
	s.Serve(lis)
}
