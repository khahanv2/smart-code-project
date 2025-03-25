package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bongg/autologin/captcha"
	"github.com/bongg/autologin/client"
	"github.com/bongg/autologin/config"
	"github.com/bongg/autologin/internal/accountprocessor"
	"github.com/bongg/autologin/logger"
	"github.com/xuri/excelize/v2"
)

// ProxyManager quản lý danh sách proxy và cung cấp chức năng lấy proxy theo luân phiên
type ProxyManager struct {
	proxies []string
	index   int
	mutex   sync.Mutex
}

// NewProxyManager tạo một instance mới để quản lý proxy
func NewProxyManager(proxyFilePath string) (*ProxyManager, error) {
	proxies, err := loadProxiesFromFile(proxyFilePath)
	if err != nil {
		return nil, err
	}

	return &ProxyManager{
		proxies: proxies,
		index:   0,
	}, nil
}

// GetNextProxy trả về proxy tiếp theo theo cơ chế luân phiên
func (pm *ProxyManager) GetNextProxy() string {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if len(pm.proxies) == 0 {
		return ""
	}

	proxy := pm.proxies[pm.index]
	pm.index = (pm.index + 1) % len(pm.proxies)

	return proxy
}

// FormatProxyURL chuyển đổi định dạng host:port:username:password thành http://username:password@host:port
func formatProxyURL(proxyStr string) string {
	parts := strings.Split(proxyStr, ":")
	if len(parts) < 4 {
		return ""
	}

	host := parts[0]
	port := parts[1]
	username := parts[2]
	password := parts[3]

	return fmt.Sprintf("http://%s:%s@%s:%s", username, password, host, port)
}

// loadProxiesFromFile tải danh sách proxy từ file
func loadProxiesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var proxies []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxyStr := strings.TrimSpace(scanner.Text())
		if proxyStr != "" {
			proxies = append(proxies, proxyStr)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return proxies, nil
}

// AccountResult lưu trữ kết quả kiểm tra tài khoản
type AccountResult struct {
	Username      string
	Password      string
	Success       bool     // Chỉ quan tâm đăng nhập có thành công không
	Balance       float64  // Số dư tài khoản dạng thập phân
	LastDeposit   float64  // Số tiền nạp gần nhất
	DepositTime   string   // Thời gian nạp tiền gần nhất (theo múi giờ HCM)
	DepositTxCode string   // Mã giao dịch nạp tiền gần nhất
	ExtraData     []string // Dữ liệu bổ sung từ file Excel
}

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
		AccountID string `json:"AccountID"`
		NickName  string `json:"NickName"`
		CookieID  string `json:"CookieID"`
		IsSuccess bool   `json:"IsSuccess"`
		Message   string `json:"Message"`
	} `json:"Data"`
	Error struct {
		Code    int    `json:"Code"`
		Message string `json:"Message"`
	} `json:"Error"`
}

// BalanceResponse cấu trúc phản hồi từ API kiểm tra số dư
type BalanceResponse struct {
	Data struct {
		BalanceAmount float64 `json:"BalanceAmount"`
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

// Biến toàn cục để lưu trữ ProxyManager
var proxyManager *ProxyManager

// Biến đếm số tài khoản
var (
	// Processor cho tài khoản
	accountProcessor *accountprocessor.AccountProcessor
)

// processAccount xử lý đăng nhập và kiểm tra thông tin một tài khoản
func processAccount(username, password string, extraData []string, resultChan chan<- AccountResult) {
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Error().Str("username", username).Interface("error", r).Msg("Có lỗi nghiêm trọng")

			// Đánh dấu tài khoản thất bại trong processor
			if accountProcessor != nil {
				accountProcessor.MarkFailed(username)
			}

			resultChan <- AccountResult{
				Username:  username,
				Password:  password,
				Success:   false,
				ExtraData: extraData,
			}
		}
	}()

	// Đánh dấu tài khoản đang được xử lý trong processor
	if accountProcessor != nil {
		accountProcessor.MarkProcessing(username)
	}

	logger.Log.Info().Str("username", username).Msg("\033[1;34m=== BẮT ĐẦU XỬ LÝ TÀI KHOẢN ===\033[0m")

	// Tạo cấu hình
	cfg := config.NewConfig(username, password)

	// Lấy và thiết lập proxy nếu có
	if proxyManager != nil {
		proxyStr := proxyManager.GetNextProxy()
		if proxyStr != "" {
			proxyURL := formatProxyURL(proxyStr)
			cfg.ProxyURL = proxyURL
			logger.Log.Info().Str("username", username).Str("proxy", proxyStr).Msg("Sử dụng proxy")
		}
	}

	// Tạo client
	cli := client.NewClient(cfg)

	// === BƯỚC 1: LẤY THÔNG TIN BAN ĐẦU ===
	logger.Log.Info().Str("username", username).Msg("Bước 1: Đang lấy thông tin từ trang chủ")
	err := cli.FetchInitialData()
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi lấy dữ liệu ban đầu")

		// Đánh dấu tài khoản thất bại trong processor
		if accountProcessor != nil {
			accountProcessor.MarkFailed(username)
		}

		resultChan <- AccountResult{
			Username:  username,
			Password:  password,
			Success:   false,
			ExtraData: extraData,
		}
		return
	}

	logger.Log.Debug().Str("username", username).Str("token", logger.TruncateToken(cli.GetToken())).Msg("Đã lấy được RequestVerificationToken")
	logger.Log.Debug().Str("username", username).Str("cookie", logger.TruncateToken(cli.GetCookie())).Msg("Đã lấy được Cookie")
	logger.Log.Debug().Str("username", username).Str("fingerIDX", cli.GetFingerIDX()).Msg("Đã tạo FingerIDX")

	// === BƯỚC 2-4: LẤY VÀ GIẢI CAPTCHA TRONG VÒNG LẶP CHO ĐẾN KHI THÀNH CÔNG ===
	var idyKey string
	logger.Log.Info().Str("username", username).Msg("\033[1;34mBắt đầu quá trình giải captcha...\033[0m")

	// Vòng lặp vô hạn cho đến khi giải được captcha
	for {
		// === BƯỚC 2: LẤY CAPTCHA ===
		logger.Log.Info().Str("username", username).Msg("Bước 2: Đang lấy Slider Captcha...")
		captchaJSON, err := cli.GetSliderCaptcha()
		if err != nil {
			logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi lấy captcha - Thử lại...")

			// Nếu timeout hoặc lỗi kết nối, thử đổi proxy
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection") {
				if proxyManager != nil {
					proxyStr := proxyManager.GetNextProxy()
					if proxyStr != "" {
						proxyURL := formatProxyURL(proxyStr)
						cfg.ProxyURL = proxyURL
						logger.Log.Info().Str("username", username).Str("proxy", proxyStr).Msg("Đã đổi proxy mới do lỗi kết nối")

						// Tạo client mới với proxy mới
						cli = client.NewClient(cfg)

						// Cần lấy lại dữ liệu ban đầu với proxy mới
						err = cli.FetchInitialData()
						if err != nil {
							logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi lấy dữ liệu ban đầu với proxy mới")
						}
					}
				}
			}

			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}

		// === BƯỚC 3: GIẢI CAPTCHA ===
		logger.Log.Info().Str("username", username).Msg("Bước 3: Đang giải Captcha...")
		startTime := time.Now()
		xPos, err := captcha.SolveCaptchaWithService(captchaJSON)
		if err != nil {
			logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi giải captcha - Thử lại...")
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}
		elapsedTime := time.Since(startTime)
		logger.Log.Info().Str("username", username).Int("xPos", xPos).Float64("elapsedTime", elapsedTime.Seconds()).Msg("Đã giải được Captcha: X = %d (%.2f giây)")

		// === BƯỚC 4: XÁC THỰC CAPTCHA ===
		logger.Log.Info().Str("username", username).Msg("Bước 4: Đang xác thực Captcha...")
		verifyResult, err := cli.CheckSliderCaptcha(xPos)
		if err != nil {
			logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi xác thực captcha - Thử lại...")
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}

		// Kiểm tra kết quả xác thực
		var response CaptchaVerifyResponse
		err = json.Unmarshal([]byte(verifyResult), &response)
		if err != nil {
			logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi parse kết quả xác thực - Thử lại...")
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}

		// Kiểm tra nếu có Message (IdyKey)
		if response.Data.Message != "" {
			idyKey = response.Data.Message
			logger.Log.Info().Str("username", username).Msg("\033[1;32mXác thực captcha thành công!\033[0m")
			break
		} else {
			logger.Log.Info().Str("username", username).Msg("Xác thực captcha thất bại - Thử lại...")
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
		}
	}

	// Thiết lập IdyKey cho client
	cli.SetIdyKey(idyKey)

	// === BƯỚC 5: ĐĂNG NHẬP (CHỈ KHI ĐÃ CÓ IDYKEY) ===
	logger.Log.Info().Str("username", username).Msg("Bước 5: Đang đăng nhập...")
	loginResult, err := cli.Login()
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi đăng nhập")

		// Nếu timeout hoặc lỗi kết nối, thử đổi proxy
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection") {
			if proxyManager != nil {
				proxyStr := proxyManager.GetNextProxy()
				if proxyStr != "" {
					proxyURL := formatProxyURL(proxyStr)
					cfg.ProxyURL = proxyURL
					logger.Log.Info().Str("username", username).Str("proxy", proxyStr).Msg("Đã đổi proxy mới do lỗi kết nối")

					// Tạo client mới với proxy mới
					cli = client.NewClient(cfg)

					// Cần lấy lại dữ liệu ban đầu và idyKey với proxy mới
					err = cli.FetchInitialData()
					if err != nil {
						logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi lấy dữ liệu ban đầu với proxy mới")
						resultChan <- AccountResult{
							Username:  username,
							Password:  password,
							Success:   false,
							ExtraData: extraData,
						}
						return
					}

					// Thử đăng nhập lại
					loginResult, err = cli.Login()
					if err != nil {
						logger.Log.Error().Str("username", username).Err(err).Msg("Vẫn lỗi sau khi đổi proxy")
						resultChan <- AccountResult{
							Username:  username,
							Password:  password,
							Success:   false,
							ExtraData: extraData,
						}
						return
					}
				}
			} else {
				resultChan <- AccountResult{
					Username:  username,
					Password:  password,
					Success:   false,
					ExtraData: extraData,
				}
				return
			}
		} else {
			resultChan <- AccountResult{
				Username:  username,
				Password:  password,
				Success:   false,
				ExtraData: extraData,
			}
			return
		}
	}

	// In ra toàn bộ JSON response để kiểm tra
	logger.Log.Debug().Str("username", username).Str("loginResult", loginResult).Msg("JSON Login response")

	// Kiểm tra kết quả đăng nhập
	var loginResponse LoginResponse
	err = json.Unmarshal([]byte(loginResult), &loginResponse)
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi parse kết quả đăng nhập")

		resultChan <- AccountResult{
			Username:      username,
			Password:      password,
			Success:       false,
			Balance:       0.0,
			LastDeposit:   0,
			DepositTime:   "",
			DepositTxCode: "",
			ExtraData:     extraData,
		}
		return
	} else {
		// Kiểm tra nếu có lỗi trong response
		if loginResponse.Error.Code > 0 || loginResponse.Error.Message != "" {
			logger.Log.Error().Str("username", username).Str("message", loginResponse.Error.Message).Msg("Đăng nhập thất bại")

			resultChan <- AccountResult{
				Username:      username,
				Password:      password,
				Success:       false,
				Balance:       0.0,
				LastDeposit:   0,
				DepositTime:   "",
				DepositTxCode: "",
				ExtraData:     extraData,
			}
			return
		}

		// Kiểm tra Data.IsSuccess nếu có (phiên bản API cũ)
		if loginResponse.Data.IsSuccess == false && loginResponse.Data.Message != "" {
			logger.Log.Error().Str("username", username).Str("message", loginResponse.Data.Message).Msg("Đăng nhập thất bại")
			resultChan <- AccountResult{
				Username:      username,
				Password:      password,
				Success:       false,
				Balance:       0.0,
				LastDeposit:   0,
				DepositTime:   "",
				DepositTxCode: "",
				ExtraData:     extraData,
			}
			return
		}

		// Kiểm tra Data.AccountID và Data.CookieID (phiên bản API mới)
		if loginResponse.Data.AccountID == "" || loginResponse.Data.CookieID == "" {
			logger.Log.Error().Str("username", username).Msg("Đăng nhập thất bại: Không có thông tin tài khoản")

			resultChan <- AccountResult{
				Username:      username,
				Password:      password,
				Success:       false,
				Balance:       0.0,
				LastDeposit:   0,
				DepositTime:   "",
				DepositTxCode: "",
				ExtraData:     extraData,
			}
			return
		}
	}

	logger.Log.Info().Str("username", username).Msg("\033[1;32mĐăng nhập thành công!\033[0m")

	// === BƯỚC 6: CẬP NHẬT THÔNG TIN SAU ĐĂNG NHẬP ===
	logger.Log.Info().Str("username", username).Msg("Bước 6: Đang cập nhật thông tin sau đăng nhập...")
	err = cli.FetchHomeAfterLogin()
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi cập nhật thông tin sau đăng nhập")
		// Vẫn thành công đăng nhập, và đã được coi là thành công
		resultChan <- AccountResult{
			Username:      username,
			Password:      password,
			Success:       true, // Đã đăng nhập thành công
			Balance:       0.0,
			LastDeposit:   0,
			DepositTime:   "",
			DepositTxCode: "",
			ExtraData:     extraData,
		}
		return
	}

	// === BƯỚC 7: KIỂM TRA SỐ DƯ ===
	logger.Log.Info().Str("username", username).Msg("Bước 7: Đang kiểm tra số dư tài khoản...")
	balanceResult, err := cli.GetMemberBalance()
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi kiểm tra số dư")
		// Vẫn thành công đăng nhập, và đã được coi là thành công
		resultChan <- AccountResult{
			Username:      username,
			Password:      password,
			Success:       true, // Vẫn thành công vì đăng nhập OK
			Balance:       0.0,
			LastDeposit:   0,
			DepositTime:   "",
			DepositTxCode: "",
			ExtraData:     extraData,
		}
		return
	}

	// In ra toàn bộ JSON để kiểm tra
	logger.Log.Debug().Str("username", username).Str("balanceResult", balanceResult).Msg("JSON Balance response")

	// Phân tích kết quả số dư
	var balanceResponse BalanceResponse
	var balance float64 = 0.0
	err = json.Unmarshal([]byte(balanceResult), &balanceResponse)
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi parse kết quả số dư")
	} else {
		// Lấy giá trị số dư trực tiếp từ cấu trúc JSON thực tế
		balance = balanceResponse.Data.BalanceAmount
		logger.Log.Info().Float64("balance", balance).Msg("\033[1;33mSố dư tài khoản: %.2f\033[0m")
	}

	// === BƯỚC 8: KIỂM TRA QUYỀN TRUY CẬP LỊCH SỬ GIAO DỊCH ===
	logger.Log.Info().Str("username", username).Msg("Bước 8: Đang kiểm tra quyền truy cập lịch sử giao dịch...")
	accessResult, err := cli.CheckTransactionAccess()
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi kiểm tra quyền truy cập")
		// Gửi kết quả với thông tin số dư
		resultChan <- AccountResult{
			Username:      username,
			Password:      password,
			Success:       true, // Vẫn thành công vì đăng nhập OK
			Balance:       balance,
			LastDeposit:   0,
			DepositTime:   "",
			DepositTxCode: "",
			ExtraData:     extraData,
		}
		return
	}

	// Kiểm tra kết quả quyền truy cập
	var accessResponse TransactionAccessResponse
	err = json.Unmarshal([]byte(accessResult), &accessResponse)
	if err != nil {
		logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi parse kết quả quyền truy cập")
	} else {
		if accessResponse.Data.IsOpen {
			logger.Log.Info().Int("limitCount", accessResponse.Data.LimitCount).Msg("Có quyền truy cập lịch sử giao dịch (Giới hạn: %d)")

			// === BƯỚC 9: LẤY LỊCH SỬ GIAO DỊCH ===
			logger.Log.Info().Str("username", username).Msg("Bước 9: Đang lấy lịch sử giao dịch...")
			historyResult, err := cli.GetTransactionHistory()
			if err != nil {
				logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi lấy lịch sử giao dịch")
			} else {
				// Phân tích kết quả lịch sử giao dịch
				var historyResponse TransactionHistoryResponse
				err = json.Unmarshal([]byte(historyResult), &historyResponse)
				if err != nil {
					logger.Log.Error().Str("username", username).Err(err).Msg("Lỗi khi parse kết quả lịch sử giao dịch")
				} else {
					// Hiển thị số lượng giao dịch
					transactionCount := len(historyResponse.Data.Data)
					logger.Log.Info().Int("transactionCount", transactionCount).Msg("Tìm thấy %d giao dịch")

					// Biến lưu trữ thông tin giao dịch nạp tiền gần nhất
					var lastDepositAmount float64 = 0
					var lastDepositTime string = ""
					var lastDepositTxCode string = ""

					// Hiển thị thông tin chi tiết cho 5 giao dịch gần nhất
					maxShow := 5
					if transactionCount < maxShow {
						maxShow = transactionCount
					}

					if transactionCount > 0 {
						logger.Log.Info().Int("maxShow", maxShow).Msg("%d giao dịch gần nhất:")
						for i := 0; i < maxShow; i++ {
							trans := historyResponse.Data.Data[i]

							// Chuyển đổi thời gian sang múi giờ HCM
							hcmTime := getHCMTime(trans.CreateTime)

							logger.Log.Info().Str("username", username).Str("transactionNumber", trans.TransactionNumber).Msg("   - Mã giao dịch: %s")
							logger.Log.Info().Str("username", username).Str("hcmTime", hcmTime).Msg("     Thời gian: %s")
							logger.Log.Info().Int("transType", trans.TransType).Msg("     Loại giao dịch: %d")
							logger.Log.Info().Float64("transactionAmount", trans.TransactionAmount).Msg("     Số tiền: %.2f")
							logger.Log.Info().Float64("balanceAmount", trans.BalanceAmount).Msg("     Số dư sau: %.2f")

							// Kiểm tra nếu là giao dịch nạp tiền thành công (TransType = 1)
							// Chú ý: Có thể cần điều chỉnh điều kiện này dựa trên mã thực tế của hệ thống
							if trans.TransType == 1 && trans.TransactionAmount > 0 {
								// Nếu chưa có giao dịch nạp tiền nào hoặc đây là giao dịch mới hơn
								if lastDepositTime == "" || lastDepositTime < hcmTime {
									lastDepositAmount = trans.TransactionAmount
									lastDepositTime = hcmTime
									lastDepositTxCode = trans.TransactionNumber
									logger.Log.Info().Str("username", username).Msg("     >>> Đây là giao dịch nạp tiền thành công gần nhất <<<")
								}
							}
						}
					}

					// Nếu tìm thấy giao dịch nạp tiền, lưu thông tin để trả về
					if lastDepositTime != "" {
						logger.Log.Info().Float64("lastDepositAmount", lastDepositAmount).Str("lastDepositTime", lastDepositTime).Str("lastDepositTxCode", lastDepositTxCode).Msg("\033[1;32mTìm thấy giao dịch nạp tiền gần nhất: %.2f VND vào %s [%s]\033[0m")

						// Gửi kết quả với thông tin số dư và giao dịch nạp tiền gần nhất
						resultChan <- AccountResult{
							Username:      username,
							Password:      password,
							Success:       true,
							Balance:       balance,
							LastDeposit:   lastDepositAmount,
							DepositTime:   lastDepositTime,
							DepositTxCode: lastDepositTxCode,
							ExtraData:     extraData,
						}
						return
					}
				}
			}
		} else {
			logger.Log.Info().Str("username", username).Msg("Không có quyền truy cập lịch sử giao dịch")
		}
	}

	// Đánh dấu tài khoản thành công trong processor
	if accountProcessor != nil {
		accountProcessor.MarkSuccess(username)
	}

	// Gửi kết quả với thông tin số dư
	resultChan <- AccountResult{
		Username:      username,
		Password:      password,
		Success:       true,
		Balance:       balance,
		LastDeposit:   0,
		DepositTime:   "",
		DepositTxCode: "",
		ExtraData:     extraData,
	}

	logger.Log.Info().Str("username", username).Msg("\033[1;32mĐĂNG NHẬP THÀNH CÔNG!\033[0m")
}

// getHCMTime chuyển đổi thời gian từ UTC sang múi giờ Hồ Chí Minh
func getHCMTime(utcTimeStr string) string {
	// Định dạng thời gian đầu vào: 2025-03-22T18:06:49.18
	t, err := time.Parse("2006-01-02T15:04:05.999", utcTimeStr)
	if err != nil {
		return utcTimeStr // Trả về nguyên bản nếu không parse được
	}

	// Đặt múi giờ Hồ Chí Minh (UTC+7)
	hcmLocation := time.FixedZone("HCM", 7*60*60)
	hcmTime := t.In(hcmLocation)

	// Định dạng thời gian đầu ra: 2025-03-22 18:06:49
	return hcmTime.Format("2006-01-02 15:04:05")
}

func main() {
	// Khởi tạo logger với pretty printing
	logger.Init("info", true)

	// Khởi tạo Processor cho tài khoản
	accountProcessor = accountprocessor.NewAccountProcessor()

	// Kiểm tra đường dẫn file Excel từ tham số dòng lệnh
	if len(os.Args) < 2 {
		logger.Log.Fatal().Msg("Cách sử dụng: batch_login <excel_file_path>")
		os.Exit(1)
	}

	// Khởi tạo ProxyManager với file proxy.txt
	var err error
	proxyManager, err = NewProxyManager("proxy.txt")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Không thể tải proxy từ file - Chạy không có proxy")
	} else {
		logger.Log.Info().Msg("Đã tải proxy thành công")
	}

	excelFilePath := os.Args[1]

	logger.Log.Info().Msg("Bắt đầu xử lý batch login từ file Excel...")

	// Kiểm tra tham số dòng lệnh
	if len(os.Args) < 2 {
		logger.Log.Info().Msg("Sử dụng: ./batch_login <file_excel.xlsx> [num_workers]")
		logger.Log.Info().Msg("  - file_excel.xlsx: File Excel chứa danh sách tài khoản")
		logger.Log.Info().Msg("  - num_workers (tùy chọn): Số luồng xử lý song song (mặc định: 1)")
		os.Exit(1)
	}

	// Đọc tham số
	excelFilePath = os.Args[1]
	maxWorkers := 1 // Mặc định 1 worker

	// Đọc số lượng workers nếu có
	if len(os.Args) > 2 {
		numWorkers, err := strconv.Atoi(os.Args[2])
		if err == nil && numWorkers > 0 {
			maxWorkers = numWorkers
		}
	}

	logger.Log.Info().Str("file", excelFilePath).Msg("Đọc file Excel")
	logger.Log.Info().Int("workers", maxWorkers).Msg("Số luồng xử lý song song")

	// Khởi động Captcha Service trước khi xử lý
	captchaErr := captcha.StartCaptchaService(9876)
	if captchaErr != nil {
		logger.Log.Warn().Err(captchaErr).Msg("Cảnh báo khi khởi động captcha service")
		logger.Log.Info().Msg("Tiếp tục xử lý với chế độ pipe...")
	}

	// Đọc file Excel
	excelFile, err := excelize.OpenFile(excelFilePath)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Không thể mở file Excel")
		os.Exit(1)
	}
	defer excelFile.Close()

	// Đọc tất cả các sheets
	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		logger.Log.Fatal().Msg("Không tìm thấy sheet nào trong file Excel")
		os.Exit(1)
	}

	// Chọn sheet đầu tiên
	sheetName := sheets[0]
	logger.Log.Info().Str("sheet", sheetName).Msg("Đọc dữ liệu từ sheet")

	// Đọc tất cả các hàng
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Không thể đọc dữ liệu từ sheet")
		os.Exit(1)
	}

	// Bỏ qua hàng đầu tiên (header)
	if len(rows) > 0 {
		rows = rows[1:]
	}

	// Khởi tạo Processor với dữ liệu từ Excel
	accountProcessor.InitializeFromExcel(rows)

	// Hiển thị thống kê ban đầu
	accountProcessor.PrintStatistics()

	// Giới hạn số lượng goroutines
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxWorkers)
	resultChan := make(chan AccountResult, len(rows))

	// Chuẩn bị file kết quả
	// Tạo thư mục kết quả nếu chưa tồn tại
	resultsDir := "results"
	if _, statErr := os.Stat(resultsDir); os.IsNotExist(statErr) {
		os.Mkdir(resultsDir, 0755)
	}

	// Chuẩn bị file Excel cho kết quả
	timestamp := time.Now().Format("20060102_150405")
	successFile := filepath.Join(resultsDir, fmt.Sprintf("success_%s.xlsx", timestamp))
	failFile := filepath.Join(resultsDir, fmt.Sprintf("fail_%s.xlsx", timestamp))

	// Tạo file Excel mới cho tài khoản thành công
	successExcel := excelize.NewFile()
	// Tạo file Excel mới cho tài khoản thất bại
	failExcel := excelize.NewFile()

	// Tạo header cho tài khoản thành công
	successHeaders := []interface{}{"Username", "Password", "Balance", "LastDeposit", "DepositTime", "DepositTxCode"}

	// Thêm header từ cột bổ sung trong file gốc
	if len(rows) > 0 && len(rows[0]) > 3 {
		for i := 3; i < len(rows[0]); i++ {
			// Sử dụng tên cột gốc nếu có, nếu không dùng Extra1, Extra2,...
			colName := fmt.Sprintf("Extra%d", i-2)
			if i < len(rows[0]) {
				successHeaders = append(successHeaders, colName)
			}
		}
	}

	// Tạo header cho tài khoản thất bại
	failHeaders := []interface{}{"Username", "Password"}

	// Thêm header từ cột bổ sung trong file gốc
	if len(rows) > 0 && len(rows[0]) > 3 {
		for i := 3; i < len(rows[0]); i++ {
			// Sử dụng tên cột gốc nếu có, nếu không dùng Extra1, Extra2,...
			colName := fmt.Sprintf("Extra%d", i-2)
			if i < len(rows[0]) {
				failHeaders = append(failHeaders, colName)
			}
		}
	}

	// Thêm header vào sheet của file thành công
	for i, header := range successHeaders {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		successExcel.SetCellValue("Sheet1", colName+"1", header)
	}

	// Thêm header vào sheet của file thất bại
	for i, header := range failHeaders {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		failExcel.SetCellValue("Sheet1", colName+"1", header)
	}

	// Tạo biến đếm số dòng trong mỗi file
	successRow := 2 // Bắt đầu từ dòng 2 (sau header)
	failRow := 2

	// Xử lý từng tài khoản
	for _, row := range rows {
		wg.Add(1)
		semaphore <- struct{}{} // Lấy token từ semaphore

		go func(rowData []string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Trả token khi hoàn thành

			// Đảm bảo có đủ cột
			if len(rowData) < 3 {
				logger.Log.Error().Msg("Bỏ qua dòng không đủ cột")
				return
			}

			username := strings.TrimSpace(rowData[1]) // Cột 2 (index 1) là tài khoản
			password := strings.TrimSpace(rowData[2]) // Cột 3 (index 2) là mật khẩu

			// Kiểm tra tài khoản hoặc mật khẩu trống
			if username == "" || password == "" {
				logger.Log.Info().Str("username", username).Msg("Bỏ qua dòng có tài khoản hoặc mật khẩu trống")
				return
			}

			// Thu thập dữ liệu thêm từ các cột khác
			var extraData []string
			if len(rowData) > 3 {
				extraData = rowData[3:]
			}

			// Xử lý tài khoản
			processAccount(username, password, extraData, resultChan)
		}(row)
	}

	// Goroutine để ghi kết quả vào file Excel
	var resultMutex sync.Mutex // Mutex để đồng bộ hóa việc ghi file

	// Tạo map để theo dõi tài khoản đã được xử lý
	processedAccounts := make(map[string]bool)

	go func() {
		for result := range resultChan {
			resultMutex.Lock() // Khóa mutex khi xử lý kết quả

			// Chỉ ghi kết quả vào file Excel mà không cập nhật biến đếm toàn cục
			// Việc đếm tài khoản đã được xử lý trong AccountProcessor
			if _, ok := processedAccounts[result.Username]; !ok {
				// Đánh dấu tài khoản này đã được xử lý
				processedAccounts[result.Username] = true
			}

			if result.Success {
				// Ghi vào file thành công
				// Chuẩn bị dữ liệu để ghi: username, password, balance, lastDeposit, depositTime, depositTxCode
				rowData := []interface{}{
					result.Username,
					result.Password,
					result.Balance,
					result.LastDeposit,
					result.DepositTime,
					result.DepositTxCode,
				}

				// Thêm dữ liệu phụ vào cuối
				for _, extra := range result.ExtraData {
					rowData = append(rowData, extra)
				}

				// Ghi dữ liệu vào sheet
				for i, cellValue := range rowData {
					colName, _ := excelize.ColumnNumberToName(i + 1)
					cellAddress := fmt.Sprintf("%s%d", colName, successRow)
					successExcel.SetCellValue("Sheet1", cellAddress, cellValue)
				}

				// Tăng số dòng cho file thành công
				successRow++
			} else {
				// Ghi vào file thất bại
				// Chuẩn bị dữ liệu để ghi: username, password
				rowData := []interface{}{result.Username, result.Password}

				// Thêm dữ liệu phụ vào cuối
				for _, extra := range result.ExtraData {
					rowData = append(rowData, extra)
				}

				// Ghi dữ liệu vào sheet
				for i, cellValue := range rowData {
					colName, _ := excelize.ColumnNumberToName(i + 1)
					cellAddress := fmt.Sprintf("%s%d", colName, failRow)
					failExcel.SetCellValue("Sheet1", cellAddress, cellValue)
				}

				// Tăng số dòng cho file thất bại
				failRow++
			}

			// Hiển thị tiến trình từ AccountProcessor
			totalAccounts := accountProcessor.GetTotalAccounts()
			successAccounts := accountProcessor.GetSuccessAccounts()
			failedAccounts := accountProcessor.GetFailedAccounts()

			// In tiến trình
			if successAccounts+failedAccounts > 0 && totalAccounts > 0 {
				percent := float64(successAccounts+failedAccounts) / float64(totalAccounts) * 100
				logger.Log.Info().Msg(fmt.Sprintf("║ Tiến trình: [%-30s] %5.1f%% ║", strings.Repeat("█", int(percent*30/100)), percent))
			} else {
				logger.Log.Info().Msg(fmt.Sprintf("║ Tiến trình: [%-30s] %5.1f%% ║", "", 0.0))
			}
			logger.Log.Info().Msg(fmt.Sprintf("║ \033[1;32m■\033[0m Thành công: %-3d \033[1;31m□\033[0m Thất bại: %-19d ║", successAccounts, failedAccounts))
			logger.Log.Info().Msg("╚══════════════════════════════════════════════╝\n")

			resultMutex.Unlock() // Mở khóa mutex khi xử lý xong
		}
	}()

	// Đợi tất cả goroutines hoàn thành
	wg.Wait()
	close(resultChan)

	// Đảm bảo tất cả dữ liệu được ghi vào file
	time.Sleep(1 * time.Second)

	// Lưu file Excel kết quả
	if err := successExcel.SaveAs(successFile); err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi lưu file kết quả thành công")
	}

	if err := failExcel.SaveAs(failFile); err != nil {
		logger.Log.Error().Err(err).Msg("Lỗi khi lưu file kết quả thất bại")
	}

	// Dừng captcha service
	captcha.StopCaptchaService()

	// Hiển thị thống kê số tài khoản
	logger.Log.Info().Msg("\n\n╔══════════════════════════════════════════════╗")
	logger.Log.Info().Msg("║            \033[1mTHỐNG KÊ TÀI KHOẢN\033[0m               ║")
	logger.Log.Info().Msg("╠══════════════════════════════════════════════╣")
	logger.Log.Info().Int("total", accountProcessor.GetTotalAccounts()).Msg(fmt.Sprintf("║ \033[1mTổng số tài khoản:\033[0m %-25d ║", accountProcessor.GetTotalAccounts()))
	logger.Log.Info().Int("success", accountProcessor.GetSuccessAccounts()).Msg(fmt.Sprintf("║ \033[1;32mSố tài khoản đăng nhập thành công:\033[0m %-13d ║", accountProcessor.GetSuccessAccounts()))
	logger.Log.Info().Int("failed", accountProcessor.GetFailedAccounts()).Msg(fmt.Sprintf("║ \033[1;31mSố tài khoản đăng nhập thất bại:\033[0m %-15d ║", accountProcessor.GetFailedAccounts()))
	logger.Log.Info().Msg("╠══════════════════════════════════════════════╣")

	// Kiểm tra tổng số tài khoản thành công và thất bại
	if (accountProcessor.GetSuccessAccounts() + accountProcessor.GetFailedAccounts()) != accountProcessor.GetTotalAccounts() {
		logger.Log.Warn().
			Int("total", accountProcessor.GetTotalAccounts()).
			Int("success", accountProcessor.GetSuccessAccounts()).
			Int("failed", accountProcessor.GetFailedAccounts()).
			Int("sum", accountProcessor.GetSuccessAccounts()+accountProcessor.GetFailedAccounts()).
			Msg("║ \033[1;33mCảnh báo: Tổng số tài khoản không khớp!\033[0m        ║")
		logger.Log.Info().Msg("╠══════════════════════════════════════════════╣")
	}

	// Tính tỷ lệ thành công
	var successRate float64 = 0
	if accountProcessor.GetTotalAccounts() > 0 {
		successRate = float64(accountProcessor.GetSuccessAccounts()) / float64(accountProcessor.GetTotalAccounts()) * 100
	}
	logger.Log.Info().Float64("rate", successRate).Msg(fmt.Sprintf("║ \033[1mTỷ lệ thành công:\033[0m %.2f%%%-25s ║", successRate, ""))

	// Hiển thị biểu đồ đơn giản bằng ký tự
	successBar := strings.Repeat("\033[1;32m■\033[0m", accountProcessor.GetSuccessAccounts())
	failedBar := strings.Repeat("\033[1;31m□\033[0m", accountProcessor.GetFailedAccounts())
	logger.Log.Info().Msg(fmt.Sprintf("║ \033[1mBiểu đồ:\033[0m %-37s ║", successBar+failedBar))
	logger.Log.Info().Msg(fmt.Sprintf("║ \033[1;32m■\033[0m Thành công: %-3d \033[1;31m□\033[0m Thất bại: %-19d ║", accountProcessor.GetSuccessAccounts(), accountProcessor.GetFailedAccounts()))
	logger.Log.Info().Msg("╚══════════════════════════════════════════════╝\n")

	logger.Log.Info().Msg("Hoàn thành kiểm tra tài khoản")
	logger.Log.Info().Str("successFile", successFile).Msg(fmt.Sprintf("Kết quả tài khoản thành công đã được lưu vào: %s", successFile))
	logger.Log.Info().Str("failFile", failFile).Msg(fmt.Sprintf("Kết quả tài khoản thất bại đã được lưu vào: %s", failFile))

	// Trước khi kết thúc, chạy kiểm tra tính nhất quán
	isValid, issues := accountProcessor.Reconcile()
	if !isValid {
		logger.Log.Warn().Msg("Phát hiện vấn đề với việc đếm tài khoản:")
		for _, issue := range issues {
			logger.Log.Warn().Msg("- " + issue)
		}
	}

	// Hiển thị thống kê cuối cùng
	accountProcessor.PrintStatistics()

	// In kết quả cuối cùng
	totalAccounts := accountProcessor.GetTotalAccounts()
	successAccounts := accountProcessor.GetSuccessAccounts()
	failedAccounts := accountProcessor.GetFailedAccounts()

	logger.Log.Info().Msg("╔══════════════════════════════════════════════╗")
	logger.Log.Info().Msg("║           KẾT QUẢ CHẠY BATCH LOGIN          ║")
	logger.Log.Info().Msg("╠══════════════════════════════════════════════╣")
	logger.Log.Info().Msgf("║ Tổng số tài khoản: %-26d ║", totalAccounts)
	logger.Log.Info().Msgf("║ Số tài khoản thành công: %-20d ║", successAccounts)
	logger.Log.Info().Msgf("║ Số tài khoản thất bại: %-22d ║", failedAccounts)
	logger.Log.Info().Msg("╚══════════════════════════════════════════════╝\n")

	logger.Log.Info().Msg("Hoàn thành kiểm tra tài khoản")
	logger.Log.Info().Str("successFile", successFile).Msg(fmt.Sprintf("Kết quả tài khoản thành công đã được lưu vào: %s", successFile))
	logger.Log.Info().Str("failFile", failFile).Msg(fmt.Sprintf("Kết quả tài khoản thất bại đã được lưu vào: %s", failFile))
}
