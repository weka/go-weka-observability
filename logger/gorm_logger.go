package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"gorm.io/gorm/logger"
)

type GormLogger struct {
	log                       *zerolog.Logger
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

func NewGormLogger(level uint, slowThreshold time.Duration, ignoreNotFound bool) *GormLogger {
	log := zerolog.New(getWriter()).
		Level(zerolog.Level(level)).
		With().
		CallerWithSkipFrameCount(6).
		Timestamp().
		Logger()
	return &GormLogger{log: &log, SlowThreshold: slowThreshold, IgnoreRecordNotFoundError: ignoreNotFound}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	var lvl zerolog.Level
	switch level {
	case logger.Info:
		lvl = zerolog.DebugLevel
	case logger.Warn:
		lvl = zerolog.WarnLevel
	case logger.Error:
		lvl = zerolog.ErrorLevel
	case logger.Silent:
		lvl = zerolog.FatalLevel
	default:
		lvl = zerolog.InfoLevel
	}
	newLog := l.log.Level(lvl)
	return &GormLogger{log: &newLog, SlowThreshold: l.SlowThreshold, IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError}
}

func (l *GormLogger) Error(ctx context.Context, msg string, opts ...interface{}) {
	l.log.Error().Msgf(msg, opts...)
}

func (l *GormLogger) Warn(ctx context.Context, msg string, opts ...interface{}) {
	l.log.Warn().Msgf(msg, opts...)
}

func (l *GormLogger) Info(ctx context.Context, msg string, opts ...interface{}) {
	l.log.Info().Msgf(msg, opts...)
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, f func() (string, int64), err error) {
	var event *zerolog.Event

	elapsed := time.Since(begin)
	if err != nil && errors.Is(err, logger.ErrRecordNotFound) && !l.IgnoreRecordNotFoundError {
		event = l.log.Info()
	} else if err != nil && !errors.Is(err, logger.ErrRecordNotFound) {
		event = l.log.Error()
	} else if elapsed > l.SlowThreshold && l.SlowThreshold != 0 {
		event = l.log.Warn()
		event.Str("SLOW", fmt.Sprintf(">= %v", l.SlowThreshold))
	} else {
		event = l.log.Debug()
	}

	event.Dur("elapsed_ms", elapsed)

	sql, rows := f()
	if rows > -1 {
		event.Int64("rows", rows)
	}
	event.Msg(sql)
}
