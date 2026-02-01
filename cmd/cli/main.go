package cli

import (
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/kardianos/service"
	"github.com/rs/zerolog"

	"github.com/Control-D-Inc/ctrld"
)

var (
	configPath        string
	configBase64      string
	daemon            bool
	listenAddress     string
	primaryUpstream   string
	secondaryUpstream string
	domains           []string
	logPath           string
	homedir           string
	cacheSize         int
	Cfg               ctrld.Config
	Verbose           int
	silent            bool
	CdUID             string
	cdOrg             string
	customHostname    string
	cdDev             bool
	iface             string
	ifaceStartStop    string
	nextdns           string
	cdUpstreamProto   string
	deactivationPin   int64
	skipSelfChecks    bool
	cleanup           bool
	startOnly         bool
	rfc1918           bool

	MainLog       atomic.Pointer[zerolog.Logger]
	ConsoleWriter zerolog.ConsoleWriter
	noConfigStart bool
)

const (
	cdUidFlagName          = "cd"
	cdOrgFlagName          = "cd-org"
	customHostnameFlagName = "custom-hostname"
	nextdnsFlagName        = "nextdns"
)

func init() {
	l := zerolog.New(io.Discard)
	MainLog.Store(&l)
}

func Main() {
	ctrld.InitConfig(v, "ctrld")
	initCLI()
	if err := rootCmd.Execute(); err != nil {
		MainLog.Load().Error().Msg(err.Error())
		os.Exit(1)
	}
}

func NormalizeLogFilePath(logFilePath string) string {
	if logFilePath == "" || filepath.IsAbs(logFilePath) || service.Interactive() {
		return logFilePath
	}
	if homedir != "" {
		return filepath.Join(homedir, logFilePath)
	}
	dir, _ := UserHomeDir()
	if dir == "" {
		return logFilePath
	}
	return filepath.Join(dir, logFilePath)
}

// initConsoleLogging initializes console logging, then storing to MainLog.
func InitConsoleLogging() {
	consoleWriter = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.TimeFormat = time.StampMilli
	})
	multi := zerolog.MultiLevelWriter(consoleWriter)
	l := MainLog.Load().Output(multi).With().Timestamp().Logger()
	MainLog.Store(&l)

	switch {
	case silent:
		zerolog.SetGlobalLevel(zerolog.NoLevel)
	case Verbose == 1:
		ctrld.ProxyLogger.Store(&l)
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case Verbose > 1:
		ctrld.ProxyLogger.Store(&l)
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.NoticeLevel)
	}
}

// initInteractiveLogging is like initLogging, but the ProxyLogger is discarded
// to be used for all interactive commands.
//
// Current log file config will also be ignored.
func initInteractiveLogging() {
	old := Cfg.Service.LogPath
	Cfg.Service.LogPath = ""
	zerolog.TimeFieldFormat = time.RFC3339 + ".000"
	InitLoggingWithBackup(false)
	Cfg.Service.LogPath = old
	l := zerolog.New(io.Discard)
	ctrld.ProxyLogger.Store(&l)
}

// initLoggingWithBackup initializes log setup base on current config.
// If doBackup is true, backup old log file with ".1" suffix.
//
// This is only used in runCmd for special handling in case of logging config
// change in cd mode. Without special reason, the caller should use initLogging
// wrapper instead of calling this function directly.
func InitLoggingWithBackup(doBackup bool) []io.Writer {
	logPath := "C:\\Program Files\\ScamJam\\scamjam-dns-watcher\\watchdog.log"
	var writers []io.Writer
	if logFilePath := NormalizeLogFilePath(logPath); logFilePath != "" {
		// Create parent directory if necessary.
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0750); err != nil {
			MainLog.Load().Error().Msgf("failed to create log path: %v", err)
			os.Exit(1)
		}

		// Default open log file in append mode.
		flags := os.O_CREATE | os.O_RDWR | os.O_APPEND
		if doBackup {
			// Backup old log file with .1 suffix.
			if err := os.Rename(logFilePath, logFilePath+oldLogSuffix); err != nil && !os.IsNotExist(err) {
				MainLog.Load().Error().Msgf("could not backup old log file: %v", err)
			} else {
				// Backup was created, set flags for truncating old log file.
				flags = os.O_CREATE | os.O_RDWR
			}
		}
		logFile, err := OpenLogFile(logFilePath, flags)
		if err != nil {
			MainLog.Load().Error().Msgf("failed to create log file: %v", err)
			os.Exit(1)
		}
		writers = append(writers, logFile)
	}
	writers = append(writers, consoleWriter)
	multi := zerolog.MultiLevelWriter(writers...)
	l := MainLog.Load().Output(multi).With().Logger()
	MainLog.Store(&l)
	// TODO: find a better way.
	ctrld.ProxyLogger.Store(&l)

	zerolog.SetGlobalLevel(zerolog.NoticeLevel)
	logLevel := "info" //TODO: read from config
	switch {
	case silent:
		zerolog.SetGlobalLevel(zerolog.NoLevel)
		return writers
	case Verbose == 1:
		logLevel = "info"
	case Verbose > 1:
		logLevel = "debug"
	}
	if logLevel == "" {
		return writers
	}
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		MainLog.Load().Warn().Err(err).Msg("could not set log level")
		return writers
	}
	zerolog.SetGlobalLevel(level)
	return writers
}

func initCache() {
	if !Cfg.Service.CacheEnable {
		return
	}
	if Cfg.Service.CacheSize == 0 {
		Cfg.Service.CacheSize = 4096
	}
}
