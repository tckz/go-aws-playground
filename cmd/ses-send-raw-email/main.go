package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/samber/lo"
	playground "github.com/tckz/go-aws-playground"
	"github.com/tckz/go-aws-playground/log"
	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

var myName = filepath.Base(os.Args[0])

var logger = zap.NewNop().Sugar()

var (
	optXrayLogLevel     = log.XrayLogLevelError
	optTo               playground.StringsFlag
	optCC               playground.StringsFlag
	optBCC              playground.StringsFlag
	optBody             = flag.String("body", "", "/path/to/mail-body.txt")
	optFrom             = flag.String("from", "", "From address")
	optSubject          = flag.String("subject", "", "Subject")
	optMessageID        = flag.String("message-id", "", "Message-ID")
	optConfigurationSet = flag.String("configuration-set", "", "SES configuration set")
)

func main() {
	_ = godotenv.Load()

	flag.Var(&optTo, "to", "To address")
	flag.Var(&optCC, "cc", "Cc address")
	flag.Var(&optBCC, "bcc", "Bcc address")
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
	if *optBody == "" {
		return errors.New("--body must be specified")
	}

	if *optFrom == "" {
		return errors.New("--from must be specified")
	}

	b, err := os.ReadFile(*optBody)
	if err != nil {
		return err
	}

	m := gomail.NewMessage()
	fromDomain := ""
	{
		addr, err := mail.ParseAddress(*optFrom)
		if err != nil {
			return fmt.Errorf("mail.ParseAddress(%s): %v", *optFrom, err)
		}
		m.SetHeader("From", *optFrom)

		// ParseAddressを通過しているので、ドメイン名部分が存在する
		i := strings.LastIndexByte(addr.Address, '@')
		fromDomain = addr.Address[i+1:]
	}
	if *optSubject != "" {
		m.SetHeader("Subject", *optSubject)
	}

	var dests []string
	appendDest := func(addrs []string) error {
		for _, to := range optTo {
			addr, err := mail.ParseAddress(to)
			if err != nil {
				return fmt.Errorf("mail.ParseAddress(%s): %v", to, err)
			}
			dests = append(dests, addr.Address)
		}
		return nil
	}
	appendDestAndHeader := func(addrs []string, field string) error {
		err := appendDest(addrs)
		if err != nil {
			return err
		}
		if len(addrs) > 0 {
			m.SetHeader(field, addrs...)
		}
		return nil
	}
	if err := appendDestAndHeader(optTo, "To"); err != nil {
		return fmt.Errorf("To: %v", err)
	}
	if err := appendDestAndHeader(optCC, "Cc"); err != nil {
		return fmt.Errorf("Cc: %v", err)
	}
	if err := appendDest(optBCC); err != nil {
		return fmt.Errorf("Bcc: %v", err)
	}

	msgid := *optMessageID
	if msgid == "" {
		msgid = "<" + uuid.NewString() + "@" + fromDomain + ">"
	}
	m.SetHeader("Message-ID", msgid)

	m.SetBody("text/plain", string(b))

	sb := &bytes.Buffer{}
	m.WriteTo(sb)
	os.Stderr.Write(sb.Bytes())

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		return fmt.Errorf("config.LoadDefaultConfig: %w", err)
	}
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	cl := ses.NewFromConfig(cfg)

	ctx, seg := xray.BeginSegment(ctx, myName)
	defer func() { seg.Close(retErr) }()

	out, err := cl.SendRawEmail(ctx, &ses.SendRawEmailInput{
		RawMessage:           &types.RawMessage{Data: sb.Bytes()},
		Destinations:         dests,
		ConfigurationSetName: lo.If(*optConfigurationSet != "", optConfigurationSet).Else(nil),
	})
	if err != nil {
		return fmt.Errorf("SendRawEmail: %v", err)
	}

	return playground.OutputAsYAML(out, os.Stdout)
}
