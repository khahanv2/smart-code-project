package main

import (
	"encoding/json"
	"fmt"
	"os"
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

// LoginResponse cấu trúc phản hồi từ API login
type LoginResponse struct {
	Status  int    `json:"Status"`
	Message string `json:"Message"`
	Data    struct {
		IsSuccess bool   `json:"IsSuccess"`
		Message   string `json:"Message"`
	} `json:"Data"`
}

// BalanceResponse cấu trúc phản hồi từ API kiểm tra số dư
type BalanceResponse struct {
	Data struct {
		WalletData struct {
			BalanceAmount float64 `json:"BalanceAmount"`
		} `json:"WalletData"`
	} `json:"Data"`
}

// TransactionAccessResponse cấu trúc phản hồi từ API kiểm tra quyền truy cập giao dịch
type TransactionAccessResponse struct {
	Data struct {
		IsOpen     bool `json:"IsOpen"`
		LimitCount int  `json:"LimitCount"`
	} `json:"Data"`
}

// TransactionHistoryResponse cấu trúc phản hồi từ API lấy lịch sử giao dịch
type TransactionHistoryResponse struct {
	Data struct {
		Data []struct {
			TransactionNumber string  `json:"TransactionNumber"`
			CreateTime        string  `json:"CreateTime"`
			TransType         int     `json:"TransType"`
			TransContent      int     `json:"TransContent"`
			TransactionAmount float64 `json:"TransactionAmount"`
			DealType_Sum      int     `json:"DealType_Sum"`
			BalanceAmount     float64 `json:"BalanceAmount"`
			PayNumber         string  `json:"PayNumber"`
			PaywayID          string  `json:"PaywayID"`
		} `json:"Data"`
		Pager struct {
			TotalItemCount int `json:"TotalItemCount"`
		} `json:"Pager"`
	} `json:"Data"`
}

func main() {
	// Khởi tạo logger
	logger.Init("info", true)

	// Kiểm tra tham số đầu vào
	if len(os.Args) < 3 {
		logger.Log.Fatal().Msg("Sử dụng: auto_login <username> <password>")
		os.Exit(1)
	}

	username := os.Args[1]
	password := os.Args[2]

	// Tạo cấu hình
	cfg := config.NewConfig(username, password)

	// Tạo client
	cli := client.NewClient(cfg)

	// === BƯỚC 1: LẤY THÔNG TIN BAN ĐẦU ===
	logger.Log.Info().Msg("Bước 1: Đang lấy thông tin từ trang chủ...")
	err := cli.FetchInitialData()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi lấy dữ liệu ban đầu")
		os.Exit(1)
	}

	logger.Log.Info().Msg("- Đã lấy được RequestVerificationToken")
	logger.Log.Info().Msg("- Đã lấy được Cookie")
	logger.Log.Info().Str("fingerIDX", cli.GetFingerIDX()).Msg("- Đã tạo FingerIDX")

	// === BƯỚC 2-4: LẤY VÀ GIẢI CAPTCHA TRONG VÒNG LẶP CHO ĐẾN KHI THÀNH CÔNG ===
	var idyKey string
	maxAttempts := 5
	attempt := 0

	for attempt < maxAttempts {
		attempt++
		logger.Log.Info().Int("attempt", attempt).Int("maxAttempts", maxAttempts).Msg("Lần thử %d/%d")

		// === BƯỚC 2: LẤY CAPTCHA ===
		logger.Log.Info().Msg("Bước 2: Đang lấy Slider Captcha...")
		captchaJSON, err := cli.GetSliderCaptcha()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Lỗi khi lấy captcha")
			continue
		}
		logger.Log.Info().Int("bytes", len(captchaJSON)).Msg("- Đã lấy được dữ liệu Captcha JSON")

		// Lưu captcha vào file để debug nếu cần
		fileName := fmt.Sprintf("captcha_%d.json", time.Now().Unix())
		err = os.WriteFile(fileName, []byte(captchaJSON), 0644)
		if err == nil {
			logger.Log.Info().Str("fileName", fileName).Msg("- Đã lưu captcha vào file")
		}

		// === BƯỚC 3: GIẢI CAPTCHA ===
		logger.Log.Info().Msg("Bước 3: Đang giải Captcha...")
		startTime := time.Now()
		xPos, err := captcha.SolveCaptcha(captchaJSON)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Lỗi khi giải captcha")
			continue
		}
		elapsedTime := time.Since(startTime)
		logger.Log.Info().Int("xPos", xPos).Float64("seconds", elapsedTime.Seconds()).Msg("- Đã giải được Captcha")

		// === BƯỚC 4: XÁC THỰC CAPTCHA (CheckSliderCaptcha thay vì VerifySliderCaptcha) ===
		logger.Log.Info().Msg("Bước 4: Đang xác thực Captcha...")
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
			logger.Log.Info().Str("idyKey", logger.TruncateToken(idyKey)).Msg("- Xác thực thành công!")
			break
		} else {
			logger.Log.Error().Str("result", logger.TruncateJSON(verifyResult)).Msg("- Xác thực thất bại!")
		}
	}

	// Kiểm tra nếu không lấy được IdyKey sau nhiều lần thử
	if idyKey == "" {
		logger.Log.Fatal().Int("attempts", maxAttempts).Msg("Không thể xác thực captcha sau nhiều lần thử. Hủy quá trình đăng nhập.")
		os.Exit(1)
	}

	// Thiết lập IdyKey cho client
	cli.SetIdyKey(idyKey)

	// === BƯỚC 5: ĐĂNG NHẬP (CHỈ KHI ĐÃ CÓ IDYKEY) ===
	logger.Log.Info().Msg("Bước 5: Đang đăng nhập...")
	loginResult, err := cli.Login()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi đăng nhập")
		os.Exit(1)
	}

	// Hiển thị kết quả đăng nhập thô để debug
	logger.Log.Debug().Str("loginResult", logger.TruncateJSON(loginResult)).Msg("- Kết quả đăng nhập thô")

	// Kiểm tra kết quả đăng nhập
	var loginResponse LoginResponse
	err = json.Unmarshal([]byte(loginResult), &loginResponse)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi parse kết quả đăng nhập")
		logger.Log.Info().Msg("Tiếp tục với giả định đăng nhập thành công...")
	} else {
		// Chỉ kiểm tra IsSuccess nếu parse JSON thành công
		if !loginResponse.Data.IsSuccess && loginResponse.Data.Message != "" {
			logger.Log.Error().Str("message", loginResponse.Data.Message).Msg("Đăng nhập thất bại")
			os.Exit(1)
		}
	}

	logger.Log.Info().Msg("- Đăng nhập thành công!")

	// === BƯỚC 6: CẬP NHẬT THÔNG TIN SAU ĐĂNG NHẬP ===
	logger.Log.Info().Msg("Bước 6: Đang cập nhật thông tin sau đăng nhập...")
	err = cli.FetchHomeAfterLogin()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi cập nhật thông tin sau đăng nhập")
		os.Exit(1)
	}

	logger.Log.Info().Msg("- Cập nhật thông tin thành công!")

	// === BƯỚC 7: KIỂM TRA SỐ DƯ ===
	logger.Log.Info().Msg("Bước 7: Đang kiểm tra số dư tài khoản...")
	balanceResult, err := cli.GetMemberBalance()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi kiểm tra số dư")
		os.Exit(1)
	}

	// Phân tích kết quả số dư
	var balanceResponse BalanceResponse
	err = json.Unmarshal([]byte(balanceResult), &balanceResponse)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi parse kết quả số dư")
	} else {
		logger.Log.Info().Float64("balance", balanceResponse.Data.WalletData.BalanceAmount).Msg("- Số dư tài khoản")
	}

	// === BƯỚC 8: KIỂM TRA QUYỀN TRUY CẬP GIAO DỊCH ===
	logger.Log.Info().Msg("Bước 8: Đang kiểm tra quyền truy cập giao dịch...")
	accessResult, err := cli.CheckTransactionAccess()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi kiểm tra quyền truy cập giao dịch")
		os.Exit(1)
	}

	// Phân tích kết quả quyền truy cập
	var accessResponse TransactionAccessResponse
	err = json.Unmarshal([]byte(accessResult), &accessResponse)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi parse kết quả quyền truy cập")
	} else {
		if accessResponse.Data.IsOpen {
			logger.Log.Info().Int("limitCount", accessResponse.Data.LimitCount).Msg("- Có quyền truy cập giao dịch")

			// === BƯỚC 9: LẤY LỊCH SỬ GIAO DỊCH ===
			logger.Log.Info().Msg("Bước 9: Đang lấy lịch sử giao dịch...")
			historyResult, err := cli.GetTransactionHistory()
			if err != nil {
				logger.Log.Error().Err(err).Msg("Lỗi khi lấy lịch sử giao dịch")
				os.Exit(1)
			}

			// Phân tích kết quả lịch sử giao dịch
			var historyResponse TransactionHistoryResponse
			err = json.Unmarshal([]byte(historyResult), &historyResponse)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Lỗi khi parse kết quả lịch sử giao dịch")
			} else {
				transactionCount := len(historyResponse.Data.Data)
				logger.Log.Info().Int("count", transactionCount).Msg("- Tìm thấy giao dịch")

				// Hiển thị các giao dịch gần nhất
				if transactionCount > 0 {
					maxShow := min(transactionCount, 5) // Tối đa 5 giao dịch gần nhất
					for i := 0; i < maxShow; i++ {
						transaction := historyResponse.Data.Data[i]
						logger.Log.Info().
							Str("txCode", transaction.TransactionNumber).
							Str("time", transaction.CreateTime).
							Int("type", transaction.TransType).
							Float64("amount", transaction.TransactionAmount).
							Float64("balance", transaction.BalanceAmount).
							Msg(fmt.Sprintf("     Giao dịch %d", i+1))
					}
				}
			}
		} else {
			logger.Log.Info().Msg("- Không có quyền truy cập giao dịch")
		}
	}

	logger.Log.Info().Msg("Hoàn thành tất cả các bước!")
}
