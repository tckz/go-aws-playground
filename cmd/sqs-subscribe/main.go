package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/aws-xray-sdk-go/header"
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
	optQueueURL     = flag.String("queue-url", "", "queue URL")
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
		if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		logger.Fatalf("%v", err)
	}
}

func run(ctx context.Context) error {
	if *optQueueURL == "" {
		return fmt.Errorf("--queue-url must be specified")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		return fmt.Errorf("config.LoadDefaultConfig: %w", err)
	}
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	cl := sqs.NewFromConfig(cfg)
	svc := &Service{
		cl: cl,
	}

	ctx = log.ToContext(ctx, logger)
	return svc.Polling(ctx)
}

type Service struct {
	cl *sqs.Client
}

func (s *Service) Polling(ctx context.Context) error {
	logger := log.Extract(ctx)
	logger.Infof("polling messages from %s", *optQueueURL)

	for {
		if err := func() (retErr error) {
			parent := ctx
			ctx, seg := xray.BeginSegment(ctx, myName+"-polling")
			defer func() {
				// 上流でキャンセルの場合は「障害」表示にしたくない
				err := retErr
				if errors.Is(err, context.Canceled) && errors.Is(parent.Err(), context.Canceled) {
					err = nil
				}
				seg.Close(err)
			}()

			out, err := s.cl.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            optQueueURL,
				AttributeNames:      []types.QueueAttributeName{types.QueueAttributeNameAll},
				MaxNumberOfMessages: 10,
			})
			if err != nil {
				return err
			}
			if err := playground.OutputAsYAML(out, os.Stdout); err != nil {
				return err
			}

			// メッセージ側のトレースIDは受信メッセージから引き継ぐのでloggerを置き換えない
			logger.With(zap.String("traceID", seg.TraceID)).Infof("%d messages", len(out.Messages))

			for _, msg := range out.Messages {
				if err := s.OnMessage(log.ToContext(ctx, logger), msg); err != nil {
					return err
				}
			}
			return nil
		}(); err != nil {
			return err
		}
	}
}

func (s *Service) OnMessage(ctx context.Context, msg types.Message) (retErr error) {
	logger := log.Extract(ctx)
	logger = logger.With(zap.String("messageID", *msg.MessageId))
	if th, ok := msg.Attributes[string(types.MessageSystemAttributeNameAWSTraceHeader)]; ok {
		c, seg := xray.NewSegmentFromHeader(ctx, myName, nil, header.FromString(th))
		defer func() { seg.Close(retErr) }()
		ctx = c
		logger = logger.With(zap.String("traceID", seg.TraceID))
	}
	ctx = log.ToContext(ctx, logger)

	logger.Infof("delete %s", *msg.ReceiptHandle)
	_, err := s.cl.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      optQueueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	return err
}
