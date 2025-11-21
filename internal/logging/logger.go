package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func Init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	Logger, _ = cfg.Build()
}

func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
