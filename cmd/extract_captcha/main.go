package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bongg/autologin/captcha"
	"github.com/bongg/autologin/client"
	"github.com/bongg/autologin/config"
	"github.com/bongg/autologin/logger"
)

// CaptchaVerifyResponse cấu trúc phản hồi từ API verify captcha
type CaptchaVerifyResponse struct {
	Data struct {
		Message string `json:"Message"`
	} `json:"Data"`
}

func main() {
	// Khởi tạo logger
	logger.Init("info", true)

	// Tạo cấu hình không cần username/password
	cfg := config.NewConfig("", "")

	// Tạo client
	cli := client.NewClient(cfg)

	// Xác định tọa độ X từ tham số dòng lệnh hoặc sử dụng giá trị mặc định
	var providedX int
	var err error
	if len(os.Args) > 1 {
		providedX, err = strconv.Atoi(os.Args[1])
		if err != nil {
			logger.Log.Fatal().Str("input", os.Args[1]).Err(err).Msg("Tọa độ X không hợp lệ")
			os.Exit(1)
		}
	}

	// === BƯỚC 1: LẤY THÔNG TIN BAN ĐẦU ===
	logger.Log.Info().Msg("=== LẤY THÔNG TIN TỪ TRANG CHỦ ===")
	err = cli.FetchInitialData()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Lỗi khi lấy dữ liệu ban đầu")
		os.Exit(1)
	}

	logger.Log.Info().Msg("User-Agent:")
	logger.Log.Info().Msg(cli.GetUserAgent())

	logger.Log.Info().Msg("RequestVerificationToken:")
	logger.Log.Info().Str("token", logger.TruncateToken(cli.GetToken())).Msg("")

	logger.Log.Info().Msg("Cookie:")
	logger.Log.Info().Str("cookie", logger.TruncateToken(cli.GetCookie())).Msg("")

	logger.Log.Info().Msg("FingerIDX:")
	logger.Log.Info().Str("fingerIDX", cli.GetFingerIDX()).Msg("")

	// === LẤY VÀ GIẢI CAPTCHA TRONG VÒNG LẶP CHO ĐẾN KHI THÀNH CÔNG ===
	var idyKey string
	maxAttempts := 5
	attempt := 0

	for attempt < maxAttempts {
		attempt++
		logger.Log.Info().Int("attempt", attempt).Msg("=== LẦN THỬ %d: LẤY SLIDER CAPTCHA ===")

		// Lấy captcha JSON
		captchaJSON, err := cli.GetSliderCaptcha()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Lỗi khi lấy captcha")
			continue
		}

		// Lưu JSON captcha vào file
		fileName := fmt.Sprintf("captcha_%d.json", time.Now().Unix())
		err = os.WriteFile(fileName, []byte(captchaJSON), 0644)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Lỗi khi lưu file captcha")
		} else {
			logger.Log.Info().Str("fileName", fileName).Msg("Đã lưu dữ liệu captcha vào file")
		}

		// Xác định tọa độ X
		var xPos int
		if providedX > 0 {
			// Sử dụng tọa độ X được cung cấp từ tham số dòng lệnh
			xPos = providedX
			logger.Log.Info().Int("xPos", xPos).Msg("Sử dụng tọa độ X được chỉ định")
		} else {
			// Giải captcha để lấy tọa độ X
			logger.Log.Info().Msg("=== GIẢI CAPTCHA ===")
			startTime := time.Now()
			xPos, err = captcha.SolveCaptcha(captchaJSON)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Lỗi khi giải captcha")
				continue
			}
			elapsedTime := time.Since(startTime)
			logger.Log.Info().Int("xPos", xPos).Float64("seconds", elapsedTime.Seconds()).Msg("Vị trí X: %d (giải trong %.2f giây)")
		}

		// === XÁC THỰC CAPTCHA ===
		logger.Log.Info().Msg("=== XÁC THỰC CAPTCHA ===")
		verifyResult, err := cli.CheckSliderCaptcha(xPos)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Lỗi khi xác thực captcha")
			continue
		}

		// Kiểm tra kết quả xác thực
		var response CaptchaVerifyResponse
		err = json.Unmarshal([]byte(verifyResult), &response)
		if err != nil {
			logger.Log.Error().Err(err).Str("data", logger.TruncateJSON(verifyResult)).Msg("Lỗi khi parse kết quả xác thực")
			continue
		}

		// Kiểm tra nếu có Message (IdyKey)
		if response.Data.Message != "" {
			idyKey = response.Data.Message
			logger.Log.Info().Str("idyKey", logger.TruncateToken(idyKey)).Msg("Xác thực thành công!")
			break
		} else {
			logger.Log.Error().Str("result", logger.TruncateJSON(verifyResult)).Msg("Xác thực thất bại!")
			// Nếu dùng tọa độ X được chỉ định từ tham số và thất bại, thì thử giải tự động
			if providedX > 0 {
				providedX = 0
				logger.Log.Info().Msg("Chuyển sang chế độ giải tự động cho lần thử tiếp theo")
			}
		}
	}

	// Hiển thị thông tin cuối cùng
	logger.Log.Info().Msg("=== THÔNG TIN CHO CURL ===")
	logger.Log.Info().Msgf("-H 'user-agent: %s'", cli.GetUserAgent())
	logger.Log.Info().Msgf("-H 'requestverificationtoken: %s'", logger.TruncateToken(cli.GetToken()))
	logger.Log.Info().Msgf("-b 'IT=%s'", logger.TruncateToken(cli.GetCookie()))

	if idyKey != "" {
		logger.Log.Info().Msg("=== THÔNG TIN ĐĂNG NHẬP ===")
		logger.Log.Info().Str("idyKey", logger.TruncateToken(idyKey)).Msg("IdyKey")
		logger.Log.Info().Str("fingerIDX", cli.GetFingerIDX()).Msg("FingerIDX")
		logger.Log.Info().Str("cookie", logger.TruncateToken(cli.GetCookie())).Msg("LocalStorgeCookie")
	} else {
		logger.Log.Error().Msg("Không thể xác thực captcha sau nhiều lần thử!")
	}
}
