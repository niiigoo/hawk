package exception

import (
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
	"time"
)

var (
	GormDefaultConfig = logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
	}
)

// NewGormLogger initializes a logger to be used with GORM
func NewGormLogger(log *logrus.Entry, config logger.Config) logger.Interface {
	return &gormLogger{
		log:    log,
		Config: config,
	}
}

type gormLogger struct {
	logger.Config
	log *logrus.Entry
}

// LogMode returns a new logger with the desired log level
func (l gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	l.LogLevel = level
	return &l
}

// Info print info
func (l gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		log := l.log.WithFields(logrus.Fields{
			"file": utils.FileWithLineNum(),
		})
		if len(data) > 0 {
			log = log.WithField("data", fmt.Sprintf("%v", data))
		}
		log.Info(msg)
	}
}

// Warn print warn messages
func (l gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		log := l.log.WithFields(logrus.Fields{
			"file": utils.FileWithLineNum(),
		})
		if len(data) > 0 {
			log = log.WithField("data", fmt.Sprintf("%v", data))
		}
		log.Warn(msg)
	}
}

// Error print error messages
func (l gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		log := l.log.WithFields(logrus.Fields{
			"file": utils.FileWithLineNum(),
		})
		if len(data) > 0 {
			log = log.WithField("data", fmt.Sprintf("%v", data))
		}
		log.Error(msg)
	}
}

// Trace prints sql message
func (l gormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	fields := logrus.Fields{"file": utils.FileWithLineNum(), "took": float64(elapsed.Nanoseconds()) / 1e6}
	log := l.log
	if err != nil {
		log = log.WithError(err)
	}

	switch {
	case err != nil && l.LogLevel >= logger.Error && (!errors.Is(err, logger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		fields["sql"] = sql
		if rows >= 0 {
			fields["rows"] = rows
		}
		log.WithFields(fields).Error("gorm: query failed")
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		sql, rows := fc()
		fields["sql"] = sql
		fields["threshold"] = l.SlowThreshold
		if rows >= 0 {
			fields["rows"] = rows
		}
		log.WithFields(fields).Info("gorm: slow query detected")
	case l.LogLevel == logger.Info:
		sql, rows := fc()
		fields["sql"] = sql
		if rows >= 0 {
			fields["rows"] = rows
		}
		log.WithFields(fields).Info("gorm: query executed")
	}
}
