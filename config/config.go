package config

import "github.com/bongg/autologin/utils"

// Config chứa thông tin cấu hình cho ứng dụng
type Config struct {
	BaseURL   string
	LoginURL  string
	Username  string
	Password  string
	UserAgent string
	ProxyURL  string // URL proxy định dạng http://username:password@host:port
}

// NewConfig tạo một cấu hình mới
func NewConfig(username, password string) *Config {
	return &Config{
		BaseURL:   "https://www.efch872.net",
		LoginURL:  "https://www.efch872.net/api/Authorize/EntryPoint88",
		Username:  username,
		Password:  password,
		UserAgent: utils.GenerateRandomUserAgent(),
		ProxyURL:  "", // Mặc định không có proxy
	}
}
