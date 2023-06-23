package log

import (
	"context"

	"go.uber.org/zap"
)

type loggerKey struct{}

func ToContext(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

var nop = zap.NewNop().Sugar()

func Extract(ctx context.Context) *zap.SugaredLogger {
	if logger, ok := ctx.Value(loggerKey{}).(*zap.SugaredLogger); ok {
		return logger
	}
	return nop
}
