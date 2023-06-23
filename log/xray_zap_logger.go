package log

import (
	"fmt"

	"github.com/aws/aws-xray-sdk-go/xraylog"
	"go.uber.org/zap"
)

var _ xraylog.Logger = (*xrayZapLogger)(nil)

type xrayZapLogger struct {
	logger      *zap.SugaredLogger
	level       xraylog.LogLevel
	fixLogLevel XrayLogLevel
}

func (l *xrayZapLogger) Log(level xraylog.LogLevel, msg fmt.Stringer) {
	chooseLogFunc := func(logger *zap.SugaredLogger, lv xraylog.LogLevel) func(...interface{}) {
		switch lv {
		case xraylog.LogLevelDebug:
			return logger.Debug
		case xraylog.LogLevelInfo:
			return logger.Info
		case xraylog.LogLevelWarn:
			return logger.Warn
		case xraylog.LogLevelError:
			return logger.Error
		}
		return logger.Info
	}

	if level >= l.level {
		var f func(...interface{})
		if l.fixLogLevel == 0 {
			f = chooseLogFunc(l.logger, level)
		} else {
			f = chooseLogFunc(l.logger.With(zap.Any("originalLevel", level)), l.fixLogLevel.Value())
		}
		f(msg.String())
	}
}

type xrayZapLoggerOption struct {
	fixLogLevel XrayLogLevel
}

type xrayZapLoggerOptionFunc func(*xrayZapLoggerOption)

func WithXrayFixLogLevel(l XrayLogLevel) xrayZapLoggerOptionFunc {
	return func(o *xrayZapLoggerOption) {
		o.fixLogLevel = l
	}
}

func NewXrayZapLogger(sl *zap.SugaredLogger, level xraylog.LogLevel, opts ...xrayZapLoggerOptionFunc) xraylog.Logger {
	opt := xrayZapLoggerOption{}
	for _, o := range opts {
		o(&opt)
	}

	return &xrayZapLogger{
		logger:      sl,
		level:       level,
		fixLogLevel: opt.fixLogLevel,
	}
}
