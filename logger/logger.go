package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var (
	// Log là logger toàn cục
	Log zerolog.Logger
	// ShowSensitiveData xác định có hiển thị dữ liệu nhạy cảm trong log không
	ShowSensitiveData bool = false
	// MaxTokenLength giới hạn độ dài của token và JSON data trong logs
	MaxTokenLength int = 30
)

// Init khởi tạo logger
func Init(level string, pretty bool) {
	// Thiết lập múi giờ Việt Nam
	zerolog.TimeFieldFormat = time.RFC3339

	// Thiết lập output
	var output io.Writer = os.Stdout

	// Nếu chạy trong terminal và yêu cầu pretty print
	if pretty {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
			FormatLevel: func(i interface{}) string {
				switch i {
				case "debug":
					return "\033[1;35m DEBUG \033[0m" // Bold magenta
				case "info":
					return "\033[1;32m INFO  \033[0m" // Bold green
				case "warn":
					return "\033[1;33m WARN  \033[0m" // Bold yellow
				case "error":
					return "\033[1;31m ERROR \033[0m" // Bold red
				case "fatal":
					return "\033[1;37;41m FATAL \033[0m" // Bold white on red background
				default:
					return "\033[1m " + strings.ToUpper(fmt.Sprintf("%-6s", i)) + " \033[0m" // Bold default
				}
			},
			FormatMessage: func(i interface{}) string {
				if i == nil || i.(string) == "" {
					return ""
				}
				return fmt.Sprintf("\033[1m%s\033[0m", i) // Bold message
			},
			FormatFieldName: func(i interface{}) string {
				return fmt.Sprintf("\033[36m%s\033[0m=", i) // Cyan field names
			},
			FormatFieldValue: func(i interface{}) string {
				return fmt.Sprintf("\033[32m%s\033[0m", i) // Green values
			},
		}
	}

	// Thiết lập log level
	logLevel := zerolog.InfoLevel
	switch strings.ToLower(level) {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	case "fatal":
		logLevel = zerolog.FatalLevel
	}

	// Tạo logger
	Log = zerolog.New(output).
		Level(logLevel).
		With().
		Timestamp().
		Caller().
		Logger()

	Log.Info().Msg("Logger đã khởi tạo")
}

// TruncateToken cắt ngắn chuỗi nếu vượt quá độ dài tối đa cho phép
func TruncateToken(data string) string {
	if len(data) <= MaxTokenLength || ShowSensitiveData {
		return data
	}

	return data[:MaxTokenLength/2] + "..." + data[len(data)-MaxTokenLength/2:]
}

// TruncateJSON cắt ngắn JSON data để không quá dài trong logs
func TruncateJSON(jsonData string) string {
	if len(jsonData) <= MaxTokenLength || ShowSensitiveData {
		return jsonData
	}

	// Hiển thị chỉ phần đầu và phần cuối
	return jsonData[:MaxTokenLength/2] + "... [truncated] ..." + jsonData[len(jsonData)-MaxTokenLength/2:]
}

// SetShowSensitiveData thiết lập chế độ hiển thị dữ liệu nhạy cảm
func SetShowSensitiveData(show bool) {
	ShowSensitiveData = show
}

// SetMaxTokenLength thiết lập độ dài tối đa cho token và JSON data
func SetMaxTokenLength(length int) {
	if length > 0 {
		MaxTokenLength = length
	}
}
