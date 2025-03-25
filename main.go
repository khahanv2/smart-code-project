package main

import (
	"fmt"
	"os"

	"github.com/bongg/autologin/client"
	"github.com/bongg/autologin/config"
	"github.com/bongg/autologin/logger"
	"github.com/bongg/autologin/utils"
)

func main() {
	// Khởi tạo logger
	logger.Init("info", true)

	// Tạo cấu hình với giá trị mặc định
	cfg := config.NewConfig("", "")

	// Tạo client
	cli := client.NewClient(cfg)

	// Lấy dữ liệu ban đầu (token, cookies)
	logger.Log.Info().Msg("Đang lấy thông tin từ trang chủ...")
	err := cli.FetchInitialData()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi lấy dữ liệu ban đầu")
		os.Exit(1)
	}

	// Hiển thị các thông tin đã lấy được
	logger.Log.Info().Msg("=== THÔNG TIN ĐÃ LẤY ĐƯỢC ===")

	logger.Log.Info().Msg("User-Agent:")
	logger.Log.Info().Msg(cli.GetUserAgent())

	logger.Log.Info().Msg("RequestVerificationToken:")
	logger.Log.Info().Str("token", logger.TruncateToken(cli.GetToken())).Msg("")

	cookieValue := cli.GetCookie()
	cookieType := "BBOSID"
	if utils.ExtractCookie(fmt.Sprintf("IT=%s", cookieValue)) != "" {
		cookieType = "IT"
	}

	logger.Log.Info().Msgf("Cookie %s:", cookieType)
	logger.Log.Info().Str("cookie", logger.TruncateToken(cookieValue)).Msg("")

	logger.Log.Info().Msg("FingerIDX (Giả lập):")
	logger.Log.Info().Str("fingerIDX", cli.GetFingerIDX()).Msg("")

	logger.Log.Info().Msg("Tất cả cookies:")
	logger.Log.Info().Str("cookies", logger.TruncateToken(cli.GetAllCookies())).Msg("")

	if idyKey := cli.GetIdyKey(); idyKey != "" {
		logger.Log.Info().Msg("IdyKey (nếu có):")
		logger.Log.Info().Str("idyKey", logger.TruncateToken(idyKey)).Msg("")
	}

	// Lấy thông tin Slider Captcha (giữ nguyên phiên)
	logger.Log.Info().Msg("=== LẤY SLIDER CAPTCHA ===")
	captchaData, err := cli.GetSliderCaptcha()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi lấy captcha")
	} else {
		logger.Log.Info().Msg("Dữ liệu Captcha (JSON):")
		logger.Log.Debug().Str("data", logger.TruncateJSON(captchaData)).Msg("")
		logger.Log.Info().Msgf("Đã nhận %d bytes dữ liệu captcha", len(captchaData))
	}

	logger.Log.Info().Msg("=== THÔNG TIN CHO CURL ===")
	logger.Log.Info().Msgf("-H 'user-agent: %s'", cli.GetUserAgent())
	logger.Log.Info().Msgf("-H 'requestverificationtoken: %s'", logger.TruncateToken(cli.GetToken()))
	if cli.GetCookie() != "" {
		cookieValue := cli.GetCookie()
		cookieType := "BBOSID"
		if utils.ExtractCookie(fmt.Sprintf("IT=%s", cookieValue)) != "" {
			cookieType = "IT"
		}
		logger.Log.Info().Msgf("-b '%s=%s'", cookieType, logger.TruncateToken(cookieValue))
	}
}
