package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/joho/godotenv"
	"github.com/samber/lo"
	playground "github.com/tckz/go-aws-playground"
	"github.com/tckz/go-aws-playground/log"
	"go.uber.org/zap"
)

var myName = filepath.Base(os.Args[0])

var (
	optTopic        = flag.String("topic", "", "topic arn")
	optMessage      = flag.String("message", "", "message to publish")
	optXrayLogLevel = log.XrayLogLevelError
)

var logger = zap.NewNop().Sugar()

func main() {
	_ = godotenv.Load()
	flag.TextVar(&optXrayLogLevel, "xray-log-level", optXrayLogLevel, "debug|info|warn|error")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logger = lo.Must(log.New()).Sugar()
	xray.SetLogger(log.NewXrayZapLogger(logger.With(zap.String("type", "xray")), optXrayLogLevel.Value(), log.WithXrayFixLogLevel(log.XrayLogLevelInfo)))

	if err := run(ctx); err != nil {
		logger.Fatalf("%v", err)
	}
}

func run(ctx context.Context) (retErr error) {
	if *optTopic == "" {
		return fmt.Errorf("--topic must be specified")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		return fmt.Errorf("config.LoadDefaultConfig: %w", err)
	}
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	cl := sns.NewFromConfig(cfg)

	ctx, seg := xray.BeginSegment(ctx, myName)
	defer func() { seg.Close(retErr) }()

	logger = logger.With(zap.String("traceID", seg.TraceID))
	logger.Info("publishing message")

	out, err := cl.Publish(ctx, &sns.PublishInput{
		Message:  optMessage,
		TopicArn: optTopic,
	})
	if err != nil {
		return err
	}

	return playground.OutputAsYAML(out, os.Stdout)
}
