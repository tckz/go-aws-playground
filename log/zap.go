package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type options struct {
	logLevel string
}

type Option func(o *options)

func WithLogLevel(lv string) Option {
	return Option(func(o *options) {
		o.logLevel = lv
	})
}

func New(opts ...Option) (*zap.Logger, error) {
	options := options{
		logLevel: "info",
	}

	for _, e := range opts {
		e(&options)
	}

	encConfig := zap.NewProductionEncoderConfig()
	encConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var al zap.AtomicLevel
	err := al.UnmarshalText([]byte(options.logLevel))
	if err != nil {
		return nil, fmt.Errorf("al.UnmarshalText: level=%s, %w", options.logLevel, err)
	}

	zc := zap.Config{
		DisableCaller:     true,
		DisableStacktrace: true,
		Level:             al,
		Development:       false,
		Encoding:          "json",
		EncoderConfig:     encConfig,
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
	}

	zl, err := zc.Build()
	if err != nil {
		return nil, fmt.Errorf("zap.Build: %w", err)
	}
	return zl, nil
}
