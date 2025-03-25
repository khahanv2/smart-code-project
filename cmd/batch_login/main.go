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

// processAccount xử lý đăng nhập và kiểm tra thông tin một tài khoản
func processAccount(username, password string, extraData []string, resultChan chan<- AccountResult) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[%s] Có lỗi nghiêm trọng: %v\n", username, r)
			resultChan <- AccountResult{
				Username:  username,
				Password:  password,
				Success:   false,
				ExtraData: extraData,
			}
		}
	}()

	fmt.Printf("[%s] Bắt đầu xử lý tài khoản\n", username)

	// Tạo cấu hình
	cfg := config.NewConfig(username, password)

	// Lấy và thiết lập proxy nếu có
	if proxyManager != nil {
		proxyStr := proxyManager.GetNextProxy()
		if proxyStr != "" {
			proxyURL := formatProxyURL(proxyStr)
			cfg.ProxyURL = proxyURL
			fmt.Printf("[%s] Sử dụng proxy: %s\n", username, proxyStr)
		}
	}

	// Tạo client
	cli := client.NewClient(cfg)

	// === BƯỚC 1: LẤY THÔNG TIN BAN ĐẦU ===
	fmt.Printf("[%s] Bước 1: Đang lấy thông tin từ trang chủ...\n", username)
	err := cli.FetchInitialData()
	if err != nil {
		fmt.Printf("[%s] Lỗi khi lấy dữ liệu ban đầu: %v\n", username, err)

		// Nếu timeout hoặc lỗi kết nối, thử đổi proxy
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection") {
			if proxyManager != nil {
				proxyStr := proxyManager.GetNextProxy()
				if proxyStr != "" {
					proxyURL := formatProxyURL(proxyStr)
					cfg.ProxyURL = proxyURL
					fmt.Printf("[%s] Đã đổi proxy mới do lỗi kết nối: %s\n", username, proxyStr)

					// Tạo client mới với proxy mới
					cli = client.NewClient(cfg)

					// Thử kết nối lại
					err = cli.FetchInitialData()
					if err != nil {
						fmt.Printf("[%s] Vẫn lỗi sau khi đổi proxy: %v\n", username, err)
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

	// === BƯỚC 2-4: LẤY VÀ GIẢI CAPTCHA TRONG VÒNG LẶP CHO ĐẾN KHI THÀNH CÔNG ===
	var idyKey string
	fmt.Printf("[%s] Bắt đầu quá trình giải captcha...\n", username)

	// Vòng lặp vô hạn cho đến khi giải được captcha
	for {
		// === BƯỚC 2: LẤY CAPTCHA ===
		fmt.Printf("[%s] Đang lấy Slider Captcha...\n", username)
		captchaJSON, err := cli.GetSliderCaptcha()
		if err != nil {
			fmt.Printf("[%s] Lỗi khi lấy captcha: %v - Thử lại...\n", username, err)

			// Nếu timeout hoặc lỗi kết nối, thử đổi proxy
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection") {
				if proxyManager != nil {
					proxyStr := proxyManager.GetNextProxy()
					if proxyStr != "" {
						proxyURL := formatProxyURL(proxyStr)
						cfg.ProxyURL = proxyURL
						fmt.Printf("[%s] Đã đổi proxy mới do lỗi kết nối: %s\n", username, proxyStr)

						// Tạo client mới với proxy mới
						cli = client.NewClient(cfg)

						// Cần lấy lại dữ liệu ban đầu với proxy mới
						err = cli.FetchInitialData()
						if err != nil {
							fmt.Printf("[%s] Lỗi khi lấy dữ liệu ban đầu với proxy mới: %v\n", username, err)
						}
					}
				}
			}

			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}

		// === BƯỚC 3: GIẢI CAPTCHA ===
		fmt.Printf("[%s] Đang giải Captcha...\n", username)
		startTime := time.Now()
		xPos, err := captcha.SolveCaptchaWithService(captchaJSON)
		if err != nil {
			fmt.Printf("[%s] Lỗi khi giải captcha: %v - Thử lại...\n", username, err)
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}
		elapsedTime := time.Since(startTime)
		fmt.Printf("[%s] Đã giải được Captcha: X = %d (%.2f giây)\n", username, xPos, elapsedTime.Seconds())

		// === BƯỚC 4: XÁC THỰC CAPTCHA ===
		fmt.Printf("[%s] Đang xác thực Captcha...\n", username)
		verifyResult, err := cli.CheckSliderCaptcha(xPos)
		if err != nil {
			fmt.Printf("[%s] Lỗi khi xác thực captcha: %v - Thử lại...\n", username, err)
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}

		// Kiểm tra kết quả xác thực
		var response CaptchaVerifyResponse
		err = json.Unmarshal([]byte(verifyResult), &response)
		if err != nil {
			fmt.Printf("[%s] Lỗi khi parse kết quả xác thực: %v - Thử lại...\n", username, err)
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
			continue
		}

		// Kiểm tra nếu có Message (IdyKey)
		if response.Data.Message != "" {
			idyKey = response.Data.Message
			fmt.Printf("[%s] Xác thực captcha thành công!\n", username)
			break
		} else {
			fmt.Printf("[%s] Xác thực captcha thất bại - Thử lại...\n", username)
			time.Sleep(1 * time.Second) // Nghỉ 1 giây trước khi thử lại
		}
	}

	// Thiết lập IdyKey cho client
	cli.SetIdyKey(idyKey)

	// === BƯỚC 5: ĐĂNG NHẬP (CHỈ KHI ĐÃ CÓ IDYKEY) ===
	fmt.Printf("[%s] Bước 5: Đang đăng nhập...\n", username)
	loginResult, err := cli.Login()
	if err != nil {
		fmt.Printf("[%s] Lỗi khi đăng nhập: %v\n", username, err)

		// Nếu timeout hoặc lỗi kết nối, thử đổi proxy
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection") {
			if proxyManager != nil {
				proxyStr := proxyManager.GetNextProxy()
				if proxyStr != "" {
					proxyURL := formatProxyURL(proxyStr)
					cfg.ProxyURL = proxyURL
					fmt.Printf("[%s] Đã đổi proxy mới do lỗi kết nối: %s\n", username, proxyStr)

					// Tạo client mới với proxy mới
					cli = client.NewClient(cfg)

					// Cần lấy lại dữ liệu ban đầu và idyKey với proxy mới
					err = cli.FetchInitialData()
					if err != nil {
						fmt.Printf("[%s] Lỗi khi lấy dữ liệu ban đầu với proxy mới: %v\n", username, err)
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
						fmt.Printf("[%s] Vẫn lỗi sau khi đổi proxy: %v\n", username, err)
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
	fmt.Printf("[%s] JSON Login response: %s\n", username, loginResult)

	// Kiểm tra kết quả đăng nhập
	var loginResponse LoginResponse
	err = json.Unmarshal([]byte(loginResult), &loginResponse)
	if err != nil {
		fmt.Printf("[%s] Lỗi khi parse kết quả đăng nhập: %v\n", username, err)
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
			fmt.Printf("[%s] Đăng nhập thất bại: %s\n", username, loginResponse.Error.Message)
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
			fmt.Printf("[%s] Đăng nhập thất bại: %s\n", username, loginResponse.Data.Message)
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
			fmt.Printf("[%s] Đăng nhập thất bại: Không có thông tin tài khoản\n", username)
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

	fmt.Printf("[%s] Đăng nhập thành công!\n", username)

	// === BƯỚC 6: CẬP NHẬT THÔNG TIN SAU ĐĂNG NHẬP ===
	fmt.Printf("[%s] Bước 6: Đang cập nhật thông tin sau đăng nhập...\n", username)
	err = cli.FetchHomeAfterLogin()
	if err != nil {
		fmt.Printf("[%s] Lỗi khi cập nhật thông tin sau đăng nhập: %v\n", username, err)
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
	fmt.Printf("[%s] Bước 7: Đang kiểm tra số dư tài khoản...\n", username)
	balanceResult, err := cli.GetMemberBalance()
	if err != nil {
		fmt.Printf("[%s] Lỗi khi kiểm tra số dư: %v\n", username, err)
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
	fmt.Printf("[%s] JSON Balance response: %s\n", username, balanceResult)

	// Phân tích kết quả số dư
	var balanceResponse BalanceResponse
	var balance float64 = 0.0
	err = json.Unmarshal([]byte(balanceResult), &balanceResponse)
	if err != nil {
		fmt.Printf("[%s] Lỗi khi parse kết quả số dư: %v\n", username, err)
	} else {
		// Lấy giá trị số dư trực tiếp từ cấu trúc JSON thực tế
		balance = balanceResponse.Data.BalanceAmount
		fmt.Printf("[%s] Số dư tài khoản: %.2f\n", username, balance)
	}

	// === BƯỚC 8: KIỂM TRA QUYỀN TRUY CẬP LỊCH SỬ GIAO DỊCH ===
	fmt.Printf("[%s] Bước 8: Đang kiểm tra quyền truy cập lịch sử giao dịch...\n", username)
	accessResult, err := cli.CheckTransactionAccess()
	if err != nil {
		fmt.Printf("[%s] Lỗi khi kiểm tra quyền truy cập: %v\n", username, err)
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
		fmt.Printf("[%s] Lỗi khi parse kết quả quyền truy cập: %v\n", username, err)
	} else {
		if accessResponse.Data.IsOpen {
			fmt.Printf("[%s] Có quyền truy cập lịch sử giao dịch (Giới hạn: %d)\n",
				username, accessResponse.Data.LimitCount)

			// === BƯỚC 9: LẤY LỊCH SỬ GIAO DỊCH ===
			fmt.Printf("[%s] Bước 9: Đang lấy lịch sử giao dịch...\n", username)
			historyResult, err := cli.GetTransactionHistory()
			if err != nil {
				fmt.Printf("[%s] Lỗi khi lấy lịch sử giao dịch: %v\n", username, err)
			} else {
				// Phân tích kết quả lịch sử giao dịch
				var historyResponse TransactionHistoryResponse
				err = json.Unmarshal([]byte(historyResult), &historyResponse)
				if err != nil {
					fmt.Printf("[%s] Lỗi khi parse kết quả lịch sử giao dịch: %v\n", username, err)
				} else {
					// Hiển thị số lượng giao dịch
					transactionCount := len(historyResponse.Data.Data)
					fmt.Printf("[%s] Tìm thấy %d giao dịch\n", username, transactionCount)

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
						fmt.Printf("[%s] %d giao dịch gần nhất:\n", username, maxShow)
						for i := 0; i < maxShow; i++ {
							trans := historyResponse.Data.Data[i]

							// Chuyển đổi thời gian sang múi giờ HCM
							hcmTime := getHCMTime(trans.CreateTime)

							fmt.Printf("[%s]   - Mã giao dịch: %s\n", username, trans.TransactionNumber)
							fmt.Printf("[%s]     Thời gian: %s\n", username, hcmTime)
							fmt.Printf("[%s]     Loại giao dịch: %d\n", username, trans.TransType)
							fmt.Printf("[%s]     Số tiền: %.2f\n", username, trans.TransactionAmount)
							fmt.Printf("[%s]     Số dư sau: %.2f\n", username, trans.BalanceAmount)

							// Kiểm tra nếu là giao dịch nạp tiền thành công (TransType = 1)
							// Chú ý: Có thể cần điều chỉnh điều kiện này dựa trên mã thực tế của hệ thống
							if trans.TransType == 1 && trans.TransactionAmount > 0 {
								// Nếu chưa có giao dịch nạp tiền nào hoặc đây là giao dịch mới hơn
								if lastDepositTime == "" || lastDepositTime < hcmTime {
									lastDepositAmount = trans.TransactionAmount
									lastDepositTime = hcmTime
									lastDepositTxCode = trans.TransactionNumber
									fmt.Printf("[%s]     >>> Đây là giao dịch nạp tiền thành công gần nhất <<<\n", username)
								}
							}
						}
					}

					// Nếu tìm thấy giao dịch nạp tiền, lưu thông tin để trả về
					if lastDepositTime != "" {
						fmt.Printf("[%s] Tìm thấy giao dịch nạp tiền gần nhất: %.2f VND vào %s [%s]\n",
							username, lastDepositAmount, lastDepositTime, lastDepositTxCode)

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
			fmt.Printf("[%s] Không có quyền truy cập lịch sử giao dịch\n", username)
		}
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
	// Kiểm tra đường dẫn file Excel từ tham số dòng lệnh
	if len(os.Args) < 2 {
		fmt.Println("Cách sử dụng: batch_login <excel_file_path>")
		os.Exit(1)
	}

	// Khởi tạo ProxyManager với file proxy.txt
	var err error
	proxyManager, err = NewProxyManager("proxy.txt")
	if err != nil {
		fmt.Printf("Cảnh báo: Không thể tải proxy từ file: %v - Chạy không có proxy\n", err)
	} else {
		fmt.Println("Đã tải proxy thành công")
	}

	excelFilePath := os.Args[1]

	fmt.Println("Bắt đầu xử lý batch login từ file Excel...")

	// Kiểm tra tham số dòng lệnh
	if len(os.Args) < 2 {
		fmt.Println("Sử dụng: ./batch_login <file_excel.xlsx> [num_workers]")
		fmt.Println("  - file_excel.xlsx: File Excel chứa danh sách tài khoản")
		fmt.Println("  - num_workers (tùy chọn): Số luồng xử lý song song (mặc định: 1)")
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

	fmt.Printf("Đọc file Excel: %s\n", excelFilePath)
	fmt.Printf("Số luồng xử lý song song: %d\n", maxWorkers)

	// Khởi động Captcha Service trước khi xử lý
	captchaErr := captcha.StartCaptchaService(9876)
	if captchaErr != nil {
		fmt.Printf("Cảnh báo khi khởi động captcha service: %v\n", captchaErr)
		fmt.Println("Tiếp tục xử lý với chế độ pipe...")
	}

	// Đọc file Excel
	excelFile, err := excelize.OpenFile(excelFilePath)
	if err != nil {
		fmt.Printf("Lỗi khi mở file Excel: %v\n", err)
		os.Exit(1)
	}
	defer excelFile.Close()

	// Lấy tất cả sheet names
	sheetNames := excelFile.GetSheetList()
	if len(sheetNames) == 0 {
		fmt.Println("Không tìm thấy sheet nào trong file Excel")
		os.Exit(1)
	}

	// Sử dụng sheet đầu tiên
	sheetName := sheetNames[0]
	fmt.Printf("Sử dụng sheet: %s\n", sheetName)

	// Đọc tất cả rows từ sheet
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		fmt.Printf("Lỗi khi đọc dữ liệu từ sheet: %v\n", err)
		os.Exit(1)
	}

	// Kiểm tra có dữ liệu không
	if len(rows) < 2 {
		fmt.Println("File Excel không có đủ dữ liệu")
		os.Exit(1)
	}

	// Bỏ qua hàng đầu tiên (header)
	if len(rows) > 1 {
		rows = rows[1:]
	}

	fmt.Printf("Tìm thấy %d tài khoản để xử lý\n", len(rows))

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

	// Giới hạn số lượng goroutines
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxWorkers)
	resultChan := make(chan AccountResult, len(rows))

	// Xử lý từng tài khoản
	for _, row := range rows {
		wg.Add(1)
		semaphore <- struct{}{} // Lấy token từ semaphore

		go func(rowData []string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Trả token khi hoàn thành

			// Đảm bảo có đủ cột
			if len(rowData) < 3 {
				fmt.Println("Bỏ qua dòng không đủ cột")
				return
			}

			username := strings.TrimSpace(rowData[1]) // Cột 2 (index 1) là tài khoản
			password := strings.TrimSpace(rowData[2]) // Cột 3 (index 2) là mật khẩu

			// Kiểm tra tài khoản hoặc mật khẩu trống
			if username == "" || password == "" {
				fmt.Println("Bỏ qua dòng có tài khoản hoặc mật khẩu trống")
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

	go func() {
		for result := range resultChan {
			resultMutex.Lock() // Khóa mutex khi xử lý kết quả

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

			resultMutex.Unlock() // Mở khóa mutex sau khi hoàn thành
		}
	}()

	// Đợi tất cả goroutines hoàn thành
	wg.Wait()
	close(resultChan)

	// Đảm bảo tất cả dữ liệu được ghi vào file
	time.Sleep(1 * time.Second)

	// Lưu file Excel kết quả
	if err := successExcel.SaveAs(successFile); err != nil {
		fmt.Printf("Lỗi khi lưu file kết quả thành công: %v\n", err)
	}

	if err := failExcel.SaveAs(failFile); err != nil {
		fmt.Printf("Lỗi khi lưu file kết quả thất bại: %v\n", err)
	}

	// Dừng captcha service
	captcha.StopCaptchaService()

	fmt.Println("Hoàn thành kiểm tra tài khoản")
	fmt.Printf("Kết quả tài khoản thành công đã được lưu vào: %s\n", successFile)
	fmt.Printf("Kết quả tài khoản thất bại đã được lưu vào: %s\n", failFile)
}
