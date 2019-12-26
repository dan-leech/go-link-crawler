package log

import "testing"

// Always passes, it just to understand how log works

func TestLog(t *testing.T) {
	logger.SetLevel(TraceLevel)

	Debug("debug message")

	var (
		a int
		b map[string]string
	)

	a = 1
	b = map[string]string{"test": "value"}

	Info("info message", a, b)
}

func TestLogf(t *testing.T) {
	logger.SetLevel(TraceLevel)

	b := map[string]string{"test": "value"}

	Infof("message: %v", b)
	Debugf("message: %v", b)
}

func TestDisabledLog(t *testing.T) {
	logger.SetLevel(ErrorLevel)

	Info("message should not be shown")
}

func TestTraceHook(t *testing.T) {
	logger.SetLevel(DebugLevel)

	Info("message without trace")
	WithTrace("one").Info("message with trace stack")
	WithTrace("one", "two", "three").Infof("message with trace stack %s", "additional")
}
