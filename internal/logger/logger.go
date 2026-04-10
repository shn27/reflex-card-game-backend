package logger

import "go.uber.org/zap"

var Logger *zap.Logger
var Sugar *zap.SugaredLogger

func InitLogger() {
	var err error
	Logger, err = zap.NewProduction()
	if err != nil {
		panic("Failed to initialize zap logger: " + err.Error())
	}
	Sugar = Logger.Sugar()
}

func CloseLogger() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
