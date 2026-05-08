package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-xray-sdk-go/v2/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/v2/xray"
	"github.com/joho/godotenv"
	"github.com/samber/lo"
	playground "github.com/tckz/go-aws-playground"
	"github.com/tckz/go-aws-playground/log"
	"go.uber.org/zap"
)

var myName = filepath.Base(os.Args[0])

var logger = zap.NewNop().Sugar()

var (
	optXrayLogLevel = log.XrayLogLevelError
	optDelimiter    = flag.String("delimiter", "/", "delimiter")
	optMaxKeys      = flag.Uint("max-keys", 1000, "max-keys")
)

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

var ErrUsage = errors.New("s3://bucket/key must be specified")

func run(ctx context.Context) (retErr error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(os.Getenv("AWS_REGION")),
		config.WithHTTPClient(xray.Client(&http.Client{})),
	)
	if err != nil {
		return fmt.Errorf("config.LoadDefaultConfig: %w", err)
	}
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	cl := s3.NewFromConfig(cfg)

	ctx, seg := xray.BeginSegment(ctx, myName)
	defer func() { seg.Close(retErr) }()

	if flag.NArg() == 0 {
		return listBuckets(ctx, cl)
	}

	return listObjects(ctx, cl)
}

func listBuckets(ctx context.Context, cl *s3.Client) error {
	var objs []types.Bucket
	paginator := s3.NewListBucketsPaginator(cl, &s3.ListBucketsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("bucket.NextPage: %w", err)
		}
		objs = append(objs, out.Buckets...)
	}

	return playground.OutputAsYAML(objs, os.Stdout)
}

func listObjects(ctx context.Context, cl *s3.Client) error {
	u, err := url.Parse(flag.Arg(0))
	if err != nil {
		return fmt.Errorf("url.Parse: %w", err)
	}
	if u.Scheme != "s3" {
		return ErrUsage
	}
	bucket := u.Host
	if bucket == "" {
		return ErrUsage
	}
	prefix := strings.TrimLeft(u.Path, "/")

	logger.Infof("bucket=%s, prefix=%s", bucket, prefix)

	var objs []any
	paginator := s3.NewListObjectsV2Paginator(cl, &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Delimiter: lo.EmptyableToPtr(*optDelimiter),
		Prefix:    lo.EmptyableToPtr(prefix),
		MaxKeys:   new(int32(*optMaxKeys)),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("obj.NextPage: %w", err)
		}
		objs = append(objs, lo.ToAnySlice(out.CommonPrefixes)...)
		objs = append(objs, lo.ToAnySlice(out.Contents)...)
	}

	return playground.OutputAsYAML(objs, os.Stdout)
}
