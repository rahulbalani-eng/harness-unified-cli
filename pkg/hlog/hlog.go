// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package hlog

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

func newHandler(w *os.File, level slog.Level) slog.Handler {
	if term.IsTerminal(int(w.Fd())) {
		return tint.NewHandler(w, &tint.Options{
			Level:      level,
			TimeFormat: "15:04:05",
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Value.Kind() == slog.KindAny {
					if _, ok := a.Value.Any().(error); ok {
						return tint.Attr(9, a)
					}
				}
				return a
			},
		})
	}
	return slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
}

var logger = slog.New(slog.DiscardHandler)

// SetDebug switches the logger to DEBUG level.
func SetDebug() {
	logger = slog.New(newHandler(os.Stderr, slog.LevelDebug))
}

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }
