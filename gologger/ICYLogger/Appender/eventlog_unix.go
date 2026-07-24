//go:build !windows

/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license.
 *
 * Copyright (C) 2023-2024 ShiLiang.Hao <newhaosl@163.com>, foobra<vipgs99@gmail.com>
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package Appender

import (
	"log/syslog"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

// syslogWriter is the Unix implementation of systemLogWriter, backed by syslog.
type syslogWriter struct {
	w *syslog.Writer
}

// newSystemLogWriter creates a syslog-backed writer for the given app name.
func newSystemLogWriter(appName string, eLogType Core.ELogType) (systemLogWriter, error) {
	w, err := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, appName)
	if err != nil {
		return nil, err
	}
	return &syslogWriter{w: w}, nil
}

// Write maps the CYGoLogger log type to the corresponding syslog priority.
func (s *syslogWriter) Write(eLogType int, msg string) error {
	switch Core.ELogType(eLogType) {
	case Core.LogTypeError, Core.LogTypeFatal:
		return s.w.Err(msg)
	case Core.LogTypeWarn:
		return s.w.Warning(msg)
	case Core.LogTypeDebug, Core.LogTypeTrace:
		return s.w.Debug(msg)
	default:
		return s.w.Info(msg)
	}
}

func (s *syslogWriter) Close() error {
	return s.w.Close()
}
