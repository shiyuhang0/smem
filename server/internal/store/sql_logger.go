package store

import (
	"context"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

var embeddingPattern = regexp.MustCompile(`\[-?\d+\.?\d*(e[+-]?\d+)?(,-?\d+\.?\d*(e[+-]?\d+)?){10,}\]`)

const disableSQLWhenNoError = true

type FilteringLogger struct {
	underlying logger.Interface
	logReads   bool
}

func NewFilteringLogger(base logger.Interface, logReads bool) *FilteringLogger {
	return &FilteringLogger{underlying: base, logReads: logReads}
}

func (l *FilteringLogger) LogMode(level logger.LogLevel) logger.Interface {
	return &FilteringLogger{underlying: l.underlying.LogMode(level), logReads: l.logReads}
}

func (l *FilteringLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	l.underlying.Info(ctx, msg, args...)
}

func (l *FilteringLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	l.underlying.Warn(ctx, msg, args...)
}

func (l *FilteringLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	l.underlying.Error(ctx, msg, args...)
}

func (l *FilteringLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if !l.logReads && err == nil {
		sql, _ := fc()
		if disableSQLWhenNoError {
			return
		}
		if isReadQuery(sql) {
			return
		}
	}
	l.underlying.Trace(ctx, begin, func() (string, int64) {
		sql, rows := fc()
		sql = embeddingPattern.ReplaceAllString(sql, "[EMBEDDING]")
		return sql, rows
	}, err)
}

func isReadQuery(sql string) bool {
	upper := strings.ToUpper(sql)
	return strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "SHOW") || strings.HasPrefix(upper, "DESCRIBE")
}
