// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mattermost/logr/v2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/utils"
	"github.com/mattermost/mattermost/server/v8/channels/utils/fileutils"
)

const (
	LogRotateSizeMB     = 100
	LogCompress         = true
	LogRotateMaxAge     = 0
	LogRotateMaxBackups = 0
	LogFilename         = "antimatter.log"
	LogMinLevelLen      = 5
	LogMinMsgLen        = 45
	LogDelim            = " "
	LogEnableCaller     = true
)

type fileLocationFunc func(string) string

func MloggerConfigFromLoggerConfig(s *model.LogSettings, configSrc LogConfigSrc, getFileFunc fileLocationFunc) (mlog.LoggerConfiguration, error) {
	cfg := make(mlog.LoggerConfiguration)

	var targetCfg mlog.TargetCfg
	var err error

	// add the simple logging config
	if *s.EnableConsole {
		targetCfg, err = makeSimpleConsoleTarget(*s.ConsoleLevel, *s.ConsoleJson, *s.EnableColor)
		if err != nil {
			return cfg, err
		}
		cfg["_defConsole"] = targetCfg
	}

	if *s.EnableFile {
		targetCfg, err = makeSimpleFileTarget(getFileFunc(*s.FileLocation), *s.FileLevel, *s.FileJson)
		if err != nil {
			return cfg, err
		}
		cfg["_defFile"] = targetCfg
	}

	if configSrc == nil {
		return cfg, nil
	}

	// add advanced logging config
	cfgAdv := configSrc.Get()
	cfg.Append(cfgAdv)

	return cfg, nil
}

func MloggerConfigFromAuditConfig(auditSettings model.ExperimentalAuditSettings, configSrc LogConfigSrc) (mlog.LoggerConfiguration, error) {
	cfg := make(mlog.LoggerConfiguration)

	var targetCfg mlog.TargetCfg
	var err error

	// add the simple audit config
	if *auditSettings.FileEnabled {
		targetCfg, err = makeSimpleFileTarget(*auditSettings.FileName, "error", true)
		if err != nil {
			return nil, err
		}

		// apply audit specific levels. Cloned so the target can never alias, and mutate,
		// the package-level slice shared with auditLevelIDs and IsAuditLevelActive.
		targetCfg.Levels = slices.Clone(basicAuditFileLevels)

		// apply audit specific formatting
		targetCfg.FormatOptions = json.RawMessage(`{"disable_timestamp": false, "disable_msg": true, "disable_stacktrace": true, "disable_level": true}`)

		cfg["_defAudit"] = targetCfg
	}

	if configSrc == nil {
		return cfg, nil
	}

	// add advanced audit config
	cfgAdv := configSrc.Get()
	cfg.Append(cfgAdv)

	return cfg, nil
}

// basicAuditFileLevels is the fixed set of levels bound to the built-in `_defAudit` file
// target created from ExperimentalAuditSettings.FileEnabled.
//
// audit-delivery is deliberately excluded: it produces one record per post per recipient,
// which is far too high volume for the basic single-file audit sink. An admin opts in by
// adding a target for it via ExperimentalAuditSettings.AdvancedLoggingJSON.
var basicAuditFileLevels = []mlog.Level{mlog.LvlAuditAPI, mlog.LvlAuditContent, mlog.LvlAuditPerms, mlog.LvlAuditCLI}

// auditLevelIDs is the set of mlog level IDs the basic audit file target consumes
// (audit-api, audit-content, audit-permissions, audit-cli).
//
// This is derived from basicAuditFileLevels rather than mlog.MLvlAuditAll on purpose: a
// config whose only audit target is bound to audit-delivery is not emitting general audit
// records, so IsAuditLoggingActive must still report false for it.
var auditLevelIDs = func() map[logr.LevelID]struct{} {
	m := make(map[logr.LevelID]struct{}, len(basicAuditFileLevels))
	for _, l := range basicAuditFileLevels {
		m[l.ID] = struct{}{}
	}
	return m
}()

// IsAuditLoggingActive reports whether the server is actually emitting audit logs to at
// least one sink, given the audit settings.
//
// It returns true when basic file auditing is enabled, or when the advanced audit logging
// config defines at least one target bound to an audit level. A valid advanced-logging
// config that routes nothing to an audit level returns false.
func IsAuditLoggingActive(auditSettings model.ExperimentalAuditSettings) bool {
	if auditSettings.FileEnabled != nil && *auditSettings.FileEnabled {
		return true
	}

	cfg := make(mlog.LoggerConfiguration)
	if err := json.Unmarshal(auditSettings.GetAdvancedLoggingConfig(), &cfg); err != nil {
		return false
	}

	for _, target := range cfg {
		for _, level := range target.Levels {
			if _, ok := auditLevelIDs[level.ID]; ok {
				return true
			}
		}
	}

	return false
}

// IsAuditLevelActive reports whether records logged at the given audit level would actually
// reach a sink.
//
// Unlike IsAuditLoggingActive, basic file auditing only counts when the requested level is
// one the built-in `_defAudit` target is bound to. Enabling
// ExperimentalAuditSettings.FileEnabled does not make audit-delivery active, for example.
func IsAuditLevelActive(auditSettings model.ExperimentalAuditSettings, level mlog.Level) bool {
	if auditSettings.FileEnabled != nil && *auditSettings.FileEnabled &&
		slices.ContainsFunc(basicAuditFileLevels, func(l mlog.Level) bool { return l.ID == level.ID }) {
		return true
	}

	cfg := make(mlog.LoggerConfiguration)
	if err := json.Unmarshal(auditSettings.GetAdvancedLoggingConfig(), &cfg); err != nil {
		return false
	}

	for _, target := range cfg {
		for _, targetLevel := range target.Levels {
			if targetLevel.ID == level.ID {
				return true
			}
		}
	}

	return false
}

func GetLogFileLocation(fileLocation string) string {
	if fileLocation == "" {
		// Match GetLogRootPath's precedence (AM_LOG_PATH/MM_LOG_PATH, then the default logs
		// directory): otherwise, with the env var set but FileLocation left at its default, the
		// log file would resolve outside the validated logging root (EnforceLogPathRoot).
		fileLocation = GetLogRootPath()
	}

	return filepath.Join(fileLocation, LogFilename)
}

// GetLogRootPath returns the root directory for all log files.
// This is used for security validation to prevent arbitrary file reads via advanced logging.
// The logging root is determined by:
// 1. AM_LOG_PATH (or MM_LOG_PATH) environment variable (if set and non-empty)
// 2. The default "logs" directory (found relative to the binary)
func GetLogRootPath() string {
	// Check environment variable
	if envPath := utils.GetServerEnv("LOG_PATH"); envPath != "" {
		absPath, err := filepath.Abs(envPath)
		if err == nil {
			return absPath
		}
	}

	// Fall back to default logs directory
	logsDir, _ := fileutils.FindDir("logs")
	absPath, err := filepath.Abs(logsDir)
	if err != nil {
		return logsDir
	}
	return absPath
}

// maxLogPathDepth caps the ancestor walk in evalSymlinksLenient. It is far beyond any real
// log path, so reaching it means the path is malformed and validation fails closed.
const maxLogPathDepth = 256

// evalSymlinksLenient resolves symlinks in path, tolerating a leaf that does not exist yet
// by resolving the deepest existing ancestor and appending the rest. An existing path is
// still resolved in full, so a symlink escaping the logging root is caught.
func evalSymlinksLenient(path string) (string, error) {
	var missing []string

	for range maxLogPathDepth {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		// EvalSymlinks reports a dangling symlink as absent, but Lstat still sees the link,
		// so an entry that exists here is unresolvable and must not be stripped as missing.
		if _, lerr := os.Lstat(path); lerr == nil {
			return "", fmt.Errorf("unresolved symlink at %s", path)
		}

		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append([]string{filepath.Base(path)}, missing...)
		path = parent
	}

	return "", fmt.Errorf("exceeded maximum directory depth of %d", maxLogPathDepth)
}

// ValidateLogFilePath validates that a log file path is within the logging root directory.
// This prevents arbitrary file read/write vulnerabilities in logging configuration.
// The logging root is determined by the AM_LOG_PATH (or MM_LOG_PATH) environment variable or the configured log directory.
func ValidateLogFilePath(filePath string, loggingRoot string) error {
	// Resolve file path to absolute
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("cannot resolve path %s: %w", filePath, err)
	}

	// Resolve symlinks to prevent bypass via symlink attacks
	absPath, err = evalSymlinksLenient(absPath)
	if err != nil {
		return fmt.Errorf("cannot resolve symlinks for %s: %w", filePath, err)
	}

	// Resolve logging root to absolute
	absRoot, err := filepath.Abs(loggingRoot)
	if err != nil {
		return fmt.Errorf("cannot resolve logging root %s: %w", loggingRoot, err)
	}

	// Must resolve the same way as absPath, or the prefix check below rejects legitimate
	// paths when an ancestor is a symlink (macOS /var -> /private/var).
	absRoot, err = evalSymlinksLenient(absRoot)
	if err != nil {
		return fmt.Errorf("cannot resolve symlinks for logging root %s: %w", loggingRoot, err)
	}

	// Ensure root has trailing separator for proper prefix matching
	// This prevents /tmp/log matching /tmp/logger
	rootWithSep := absRoot
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}

	// Check if file is within the logging root
	// Allow exact match (absPath == absRoot) or proper prefix match
	if absPath != absRoot && !strings.HasPrefix(absPath, rootWithSep) {
		return fmt.Errorf("path %s is outside logging root %s", filePath, absRoot)
	}

	return nil
}

// LogPathOutsideRootWarning is the message logged when a log or audit file path resolves
// outside the logging root and the server is not configured to reject it.
const LogPathOutsideRootWarning = "Log file path in logging config is outside the logging root directory. " +
	"Enforcement is on by default; it was turned off with the EnforceLogPathRoot feature flag (AM_FEATUREFLAGS_ENFORCELOGPATHROOT=false), " +
	"which lets the server start with this path for now. " +
	"To fix, set the AM_LOG_PATH environment variable (or the older MM_LOG_PATH) to a parent directory containing all log paths, or move log files into the logging root."

// LogPathFallbackWarning is the message logged when log path enforcement is on and log or audit file targets of the
// configuration sit outside the logging root: the server starts, writing those targets in the logging root instead.
const LogPathFallbackWarning = "Log or audit file paths in the configuration are outside the logging root directory, so those files are written in the logging root instead " +
	"(a configuration from an older version or from Mattermost may keep its logs elsewhere). " +
	"To keep them where they are, set the AM_LOG_PATH environment variable (or the older MM_LOG_PATH) to a parent directory containing all log paths; " +
	"otherwise point the settings named here at the logging root. Configuration changes made with a path outside the logging root are refused."

// ConfineLogPaths returns a copy of cfg in which every log and audit file target that ValidateLogPaths rejects is
// moved into loggingRoot, and the targets it moved. LogSettings.FileLocation becomes loggingRoot (the log file keeps
// its name); the audit file and the file targets of advanced logging keep their file name in loggingRoot. cfg itself
// is left unchanged: the configuration still holds the paths the admin set, the loggers are set up from the copy.
func ConfineLogPaths(cfg *model.Config, loggingRoot string) (*model.Config, []string) {
	if cfg == nil {
		return nil, nil
	}
	confined := cfg.Clone()
	var moved []string

	if model.SafeDereference(cfg.LogSettings.EnableFile) {
		logFile := GetLogFileLocation(model.SafeDereference(cfg.LogSettings.FileLocation))
		if ValidateLogFilePath(logFile, loggingRoot) != nil {
			confined.LogSettings.FileLocation = model.NewPointer(loggingRoot)
			moved = append(moved, "LogSettings.FileLocation")
		}
	}

	if loggingJSON, targets := confineAdvancedLoggingPaths(cfg.LogSettings.AdvancedLoggingJSON, "LogSettings.AdvancedLoggingJSON", loggingRoot); len(targets) > 0 {
		confined.LogSettings.AdvancedLoggingJSON = loggingJSON
		moved = append(moved, targets...)
	}

	if model.SafeDereference(cfg.ExperimentalAuditSettings.FileEnabled) {
		auditFile := model.SafeDereference(cfg.ExperimentalAuditSettings.FileName)
		if ValidateLogFilePath(auditFile, loggingRoot) != nil {
			confined.ExperimentalAuditSettings.FileName = model.NewPointer(filepath.Join(loggingRoot, confinedFileName(auditFile, "audit.log")))
			moved = append(moved, "ExperimentalAuditSettings.FileName")
		}
	}

	if loggingJSON, targets := confineAdvancedLoggingPaths(cfg.ExperimentalAuditSettings.AdvancedLoggingJSON, "ExperimentalAuditSettings.AdvancedLoggingJSON", loggingRoot); len(targets) > 0 {
		confined.ExperimentalAuditSettings.AdvancedLoggingJSON = loggingJSON
		moved = append(moved, targets...)
	}

	return confined, moved
}

// confinedFileName gives the name a file moved into the logging root keeps: its own, or fallback for a path that
// names no file.
func confinedFileName(path, fallback string) string {
	name := filepath.Base(strings.TrimSpace(path))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return fallback
	}
	return name
}

// confineAdvancedLoggingPaths moves the file targets of an advanced logging config that sit outside loggingRoot into
// it, keeping their file names, and returns the rewritten config and the targets moved (none: loggingJSON as is).
// Configs validateAdvancedLoggingPaths skips (unparseable, or a file reference) are left as they are.
func confineAdvancedLoggingPaths(loggingJSON json.RawMessage, configSection string, loggingRoot string) (json.RawMessage, []string) {
	if utils.IsEmptyJSON(loggingJSON) {
		return loggingJSON, nil
	}

	logCfg := make(mlog.LoggerConfiguration)
	if err := json.Unmarshal(loggingJSON, &logCfg); err != nil {
		return loggingJSON, nil
	}

	var moved []string
	for _, targetName := range slices.Sorted(maps.Keys(logCfg)) {
		target := logCfg[targetName]
		if !strings.EqualFold(target.Type, "file") {
			continue
		}

		var options map[string]any
		if err := json.Unmarshal(target.Options, &options); err != nil || options == nil {
			continue
		}
		filename, _ := options["filename"].(string)
		if ValidateLogFilePath(filename, loggingRoot) == nil {
			continue
		}

		options["filename"] = filepath.Join(loggingRoot, confinedFileName(filename, targetName+".log"))
		rewritten, err := json.Marshal(options)
		if err != nil {
			continue
		}
		target.Options = rewritten
		logCfg[targetName] = target
		moved = append(moved, fmt.Sprintf("%s target %q", configSection, targetName))
	}

	if len(moved) == 0 {
		return loggingJSON, nil
	}
	rewritten, err := json.Marshal(logCfg)
	if err != nil {
		return loggingJSON, nil
	}
	return rewritten, moved
}

// IsLogPathEnforcementEnabled reports whether a log path outside the logging root should be
// treated as fatal rather than merely logged. Nil-safe: cfg may carry no feature flags at all.
func IsLogPathEnforcementEnabled(cfg *model.Config) bool {
	return cfg != nil && cfg.FeatureFlags != nil && cfg.FeatureFlags.EnforceLogPathRoot
}

// ValidateLogPaths checks every log and audit file path in cfg against loggingRoot, returning an
// error naming the first one that resolves outside it. The fields checked are
// LogSettings.FileLocation, LogSettings.AdvancedLoggingJSON, ExperimentalAuditSettings.FileName
// and ExperimentalAuditSettings.AdvancedLoggingJSON.
//
// loggingRoot is a parameter rather than a call to GetLogRootPath so a caller holding a
// per-instance override (see PlatformService.getLogRootPath) can supply its own root.
func ValidateLogPaths(cfg *model.Config, loggingRoot string) error {
	if cfg == nil {
		return nil
	}

	// LogSettings.FileLocation feeds the built-in `_defFile` target.
	if model.SafeDereference(cfg.LogSettings.EnableFile) {
		logFile := GetLogFileLocation(model.SafeDereference(cfg.LogSettings.FileLocation))
		if err := ValidateLogFilePath(logFile, loggingRoot); err != nil {
			return fmt.Errorf("LogSettings.FileLocation: %w", err)
		}
	}

	if err := validateAdvancedLoggingPaths(cfg.LogSettings.AdvancedLoggingJSON, "LogSettings.AdvancedLoggingJSON", loggingRoot); err != nil {
		return err
	}

	// ExperimentalAuditSettings.FileName feeds the built-in `_defAudit` target.
	if model.SafeDereference(cfg.ExperimentalAuditSettings.FileEnabled) {
		auditFile := model.SafeDereference(cfg.ExperimentalAuditSettings.FileName)
		if err := ValidateLogFilePath(auditFile, loggingRoot); err != nil {
			return fmt.Errorf("ExperimentalAuditSettings.FileName: %w", err)
		}
	}

	return validateAdvancedLoggingPaths(cfg.ExperimentalAuditSettings.AdvancedLoggingJSON, "ExperimentalAuditSettings.AdvancedLoggingJSON", loggingRoot)
}

// logFileTarget is a file a log or audit target of a configuration writes to.
type logFileTarget struct {
	setting string // the configuration setting it comes from, as ValidateLogPaths names it
	target  string // the advanced logging target's name; empty for the basic log and audit files
	path    string
}

// describe names the target as ValidateLogPaths' errors do.
func (t logFileTarget) describe() string {
	if t.target == "" {
		return t.setting
	}
	return fmt.Sprintf("%s target %q", t.setting, t.target)
}

// logFileTargets lists the files cfg's active log and audit targets write to, in ValidateLogPaths' order.
func logFileTargets(cfg *model.Config) []logFileTarget {
	if cfg == nil {
		return nil
	}
	var targets []logFileTarget
	if model.SafeDereference(cfg.LogSettings.EnableFile) {
		targets = append(targets, logFileTarget{setting: "LogSettings.FileLocation", path: GetLogFileLocation(model.SafeDereference(cfg.LogSettings.FileLocation))})
	}
	targets = append(targets, advancedLoggingFileTargets(cfg.LogSettings.AdvancedLoggingJSON, "LogSettings.AdvancedLoggingJSON")...)
	if model.SafeDereference(cfg.ExperimentalAuditSettings.FileEnabled) {
		targets = append(targets, logFileTarget{setting: "ExperimentalAuditSettings.FileName", path: model.SafeDereference(cfg.ExperimentalAuditSettings.FileName)})
	}
	return append(targets, advancedLoggingFileTargets(cfg.ExperimentalAuditSettings.AdvancedLoggingJSON, "ExperimentalAuditSettings.AdvancedLoggingJSON")...)
}

// advancedLoggingFileTargets lists the file targets of an advanced logging config, skipping what
// validateAdvancedLoggingPaths skips.
func advancedLoggingFileTargets(loggingJSON json.RawMessage, setting string) []logFileTarget {
	if utils.IsEmptyJSON(loggingJSON) {
		return nil
	}
	logCfg := make(mlog.LoggerConfiguration)
	if err := json.Unmarshal(loggingJSON, &logCfg); err != nil {
		return nil
	}
	var targets []logFileTarget
	for _, targetName := range slices.Sorted(maps.Keys(logCfg)) {
		target := logCfg[targetName]
		if !strings.EqualFold(target.Type, "file") {
			continue
		}
		var fileOption struct {
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(target.Options, &fileOption); err != nil {
			continue
		}
		targets = append(targets, logFileTarget{setting: setting, target: targetName, path: fileOption.Filename})
	}
	return targets
}

// ValidateLogPathChanges is ValidateLogPaths for a configuration change from oldCfg to newCfg: a file outside
// loggingRoot is refused only when the change brings it in. A file the same setting of oldCfg already writes to
// (a configuration upgraded from an older version or from Mattermost that keeps its logs in /var/log/mattermost,
// say) is left alone, so the admin can still save other settings; the loggers write it in the logging root anyway
// (ConfineLogPaths). A new path, a target newly turned on, or a path moved to another setting is still refused.
func ValidateLogPathChanges(oldCfg, newCfg *model.Config, loggingRoot string) error {
	carried := map[string]map[string]bool{}
	for _, target := range logFileTargets(oldCfg) {
		if carried[target.setting] == nil {
			carried[target.setting] = map[string]bool{}
		}
		carried[target.setting][comparableLogPath(target.path)] = true
	}

	for _, target := range logFileTargets(newCfg) {
		err := ValidateLogFilePath(target.path, loggingRoot)
		if err == nil || carried[target.setting][comparableLogPath(target.path)] {
			continue
		}
		return fmt.Errorf("%s: %w", target.describe(), err)
	}
	return nil
}

// comparableLogPath gives the form two log paths are compared in: absolute and cleaned, without resolving symlinks
// (the value compared is the configured one).
func comparableLogPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// validateAdvancedLoggingPaths validates the file targets of an advanced logging config.
//
// Unparseable JSON is skipped rather than reported: it defines no file target, so it cannot write
// anywhere, and LogSettings.isValid/ConfigureLogger already reject it with a better message.
func validateAdvancedLoggingPaths(loggingJSON json.RawMessage, configSection string, loggingRoot string) error {
	if utils.IsEmptyJSON(loggingJSON) {
		return nil
	}

	logCfg := make(mlog.LoggerConfiguration)
	if err := json.Unmarshal(loggingJSON, &logCfg); err != nil {
		return nil
	}

	// map iteration order is random; sort so the reported target is deterministic
	for _, targetName := range slices.Sorted(maps.Keys(logCfg)) {
		target := logCfg[targetName]
		if !strings.EqualFold(target.Type, "file") {
			continue
		}

		var fileOption struct {
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(target.Options, &fileOption); err != nil {
			continue
		}

		if err := ValidateLogFilePath(fileOption.Filename, loggingRoot); err != nil {
			return fmt.Errorf("%s target %q: %w", configSection, targetName, err)
		}
	}

	return nil
}

func makeSimpleConsoleTarget(level string, outputJSON bool, color bool) (mlog.TargetCfg, error) {
	levels, err := stdLevels(level)
	if err != nil {
		return mlog.TargetCfg{}, err
	}

	target := mlog.TargetCfg{
		Type:         "console",
		Levels:       levels,
		Options:      json.RawMessage(`{"out": "stdout"}`),
		MaxQueueSize: 1000,
	}

	if outputJSON {
		target.Format = "json"
		target.FormatOptions = makeJSONFormatOptions()
	} else {
		target.Format = "plain"
		target.FormatOptions = makePlainFormatOptions(color)
	}
	return target, nil
}

func makeSimpleFileTarget(filename string, level string, json bool) (mlog.TargetCfg, error) {
	levels, err := stdLevels(level)
	if err != nil {
		return mlog.TargetCfg{}, err
	}

	fileOpts, err := makeFileOptions(filename)
	if err != nil {
		return mlog.TargetCfg{}, fmt.Errorf("cannot encode file options: %w", err)
	}

	target := mlog.TargetCfg{
		Type:         "file",
		Levels:       levels,
		Options:      fileOpts,
		MaxQueueSize: 1000,
	}

	if json {
		target.Format = "json"
		target.FormatOptions = makeJSONFormatOptions()
	} else {
		target.Format = "plain"
		target.FormatOptions = makePlainFormatOptions(false)
	}
	return target, nil
}

func stdLevels(level string) ([]mlog.Level, error) {
	stdLevel, err := stringToStdLevel(level)
	if err != nil {
		return nil, err
	}

	var levels []mlog.Level
	for _, l := range mlog.StdAll {
		if l.ID <= stdLevel.ID {
			levels = append(levels, l)
		}
	}
	return levels, nil
}

func stringToStdLevel(level string) (mlog.Level, error) {
	level = strings.ToLower(level)
	for _, l := range mlog.StdAll {
		if l.Name == level {
			return l, nil
		}
	}
	return mlog.Level{}, fmt.Errorf("%s is not a standard level", level)
}

func makeJSONFormatOptions() json.RawMessage {
	str := fmt.Sprintf(`{"enable_caller": %t}`, LogEnableCaller)
	return json.RawMessage(str)
}

func makePlainFormatOptions(enableColor bool) json.RawMessage {
	str := fmt.Sprintf(`{"delim": "%s", "min_level_len": %d, "min_msg_len": %d, "enable_color": %t, "enable_caller": %t}`,
		LogDelim, LogMinLevelLen, LogMinMsgLen, enableColor, LogEnableCaller)
	return json.RawMessage(str)
}

func makeFileOptions(filename string) (json.RawMessage, error) {
	opts := struct {
		Filename    string `json:"filename"`
		Max_size    int    `json:"max_size"`
		Max_age     int    `json:"max_age"`
		Max_backups int    `json:"max_backups"`
		Compress    bool   `json:"compress"`
	}{
		Filename:    filename,
		Max_size:    LogRotateSizeMB,
		Max_age:     LogRotateMaxAge,
		Max_backups: LogRotateMaxBackups,
		Compress:    LogCompress,
	}

	b, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(b), nil
}
