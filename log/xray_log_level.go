package log

import (
	"encoding"
	"fmt"
	"strings"

	"github.com/aws/aws-xray-sdk-go/xraylog"
)

var xrayLogLevelText = make(map[string]xraylog.LogLevel)

func init() {
	for _, e := range []xraylog.LogLevel{
		xraylog.LogLevelDebug,
		xraylog.LogLevelInfo,
		xraylog.LogLevelWarn,
		xraylog.LogLevelError,
	} {
		xrayLogLevelText[strings.ToLower(e.String())] = e
	}
}

// for convenience
var (
	XrayLogLevelDebug = XrayLogLevel(xraylog.LogLevelDebug)
	XrayLogLevelInfo  = XrayLogLevel(xraylog.LogLevelInfo)
	XrayLogLevelWarn  = XrayLogLevel(xraylog.LogLevelWarn)
	XrayLogLevelError = XrayLogLevel(xraylog.LogLevelError)
)

var _ encoding.TextMarshaler = (*XrayLogLevel)(nil)
var _ encoding.TextUnmarshaler = (*XrayLogLevel)(nil)

type XrayLogLevel xraylog.LogLevel

func (l *XrayLogLevel) UnmarshalText(b []byte) error {
	s := strings.ToLower(string(b))
	v, ok := xrayLogLevelText[s]
	if !ok {
		return fmt.Errorf("unknown log level: %s", s)
	}
	*l = XrayLogLevel(v)
	return nil
}

func (l XrayLogLevel) MarshalText() (text []byte, err error) {
	return []byte(strings.ToLower(xraylog.LogLevel(l).String())), nil
}

func (l XrayLogLevel) Value() xraylog.LogLevel {
	return xraylog.LogLevel(l)
}
