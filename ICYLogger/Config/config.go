/*
 * CYGoLogger License
 * -----------
 * ... (MIT license) ...
 */

// Package Config provides backward-compatible re-exports from Core.CYLoggerConfig.
// Users should prefer importing the Core package directly.
package Config

import (
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

// CYLoggerConfig re-exports Core.CYLoggerConfig for backward compatibility.
type CYLoggerConfig = Core.CYLoggerConfig

// GetCYLoggerConfigInstance re-exports Core.GetCYLoggerConfigInstance.
func GetCYLoggerConfigInstance() *CYLoggerConfig {
	return Core.GetCYLoggerConfigInstance()
}
