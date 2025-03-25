package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bongg/autologin/captcha"
	"github.com/bongg/autologin/client"
	"github.com/bongg/autologin/config"
)

// CaptchaVerifyResponse cấu trúc phản hồi từ API verify captcha
type CaptchaVerifyResponse struct {
	Data struct {
		Message string `json:"Message"`
	} `json:"Data"`
}

// LoginResponse cấu trúc phản hồi từ API login
type LoginResponse struct {
	Status  int `json:"Status"`
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
		IsOpen      bool `json:"IsOpen"`
		LimitCount  int  `json:"LimitCount"`
	} `json:"Data"`
}

// TransactionHistoryResponse cấu trúc phản hồi từ API lấy lịch sử giao dịch
type TransactionHistoryResponse struct {
	Data struct {
		Data []struct {
			TransactionNumber  string  `json:"TransactionNumber"`
			CreateTime         string  `json:"CreateTime"`
			TransType          int     `json:"TransType"`
			TransContent       int     `json:"TransContent"`
			TransactionAmount  float64 `json:"TransactionAmount"`
			DealType_Sum       int     `json:"DealType_Sum"`
			BalanceAmount      float64 `json:"BalanceAmount"`
			PayNumber          string  `json:"PayNumber"`
			PaywayID           string  `json:"PaywayID"`
		} `json:"Data"`
		Pager struct {
			TotalItemCount int `json:"TotalItemCount"`
		} `json:"Pager"`
	} `json:"Data"`
}

func main() {
	// Kiểm tra tham số đầu vào
	if len(os.Args) < 3 {
		fmt.Println("Sử dụng: auto_login <username> <password>")
		os.Exit(1)
	}

	username := os.Args[1]
	password := os.Args[2]

	// Tạo cấu hình
	cfg := config.NewConfig(username, password)

	// Tạo client
	cli := client.NewClient(cfg)

	// === BƯỚC 1: LẤY THÔNG TIN BAN ĐẦU ===
	fmt.Println("Bước 1: Đang lấy thông tin từ trang chủ...")
	err := cli.FetchInitialData()
	if err != nil {
		fmt.Printf("Lỗi khi lấy dữ liệu ban đầu: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("- Đã lấy được RequestVerificationToken\n")
	fmt.Printf("- Đã lấy được Cookie\n")
	fmt.Printf("- Đã tạo FingerIDX: %s\n", cli.GetFingerIDX())

	// === BƯỚC 2-4: LẤY VÀ GIẢI CAPTCHA TRONG VÒNG LẶP CHO ĐẾN KHI THÀNH CÔNG ===
	var idyKey string
	maxAttempts := 5
	attempt := 0

	for attempt < maxAttempts {
		attempt++
		fmt.Printf("\nLần thử %d/%d\n", attempt, maxAttempts)
		
		// === BƯỚC 2: LẤY CAPTCHA ===
		fmt.Println("Bước 2: Đang lấy Slider Captcha...")
		captchaJSON, err := cli.GetSliderCaptcha()
		if err != nil {
			fmt.Printf("Lỗi khi lấy captcha: %v\n", err)
			continue
		}
		fmt.Printf("- Đã lấy được dữ liệu Captcha JSON (%d bytes)\n", len(captchaJSON))

		// Lưu captcha vào file để debug nếu cần
		fileName := fmt.Sprintf("captcha_%d.json", time.Now().Unix())
		err = os.WriteFile(fileName, []byte(captchaJSON), 0644)
		if err == nil {
			fmt.Printf("- Đã lưu captcha vào file: %s\n", fileName)
		}

		// === BƯỚC 3: GIẢI CAPTCHA ===
		fmt.Println("\nBước 3: Đang giải Captcha...")
		startTime := time.Now()
		xPos, err := captcha.SolveCaptcha(captchaJSON)
		if err != nil {
			fmt.Printf("Lỗi khi giải captcha: %v\n", err)
			continue
		}
		elapsedTime := time.Since(startTime)
		fmt.Printf("- Đã giải được Captcha: X = %d (%.2f giây)\n", xPos, elapsedTime.Seconds())

		// === BƯỚC 4: XÁC THỰC CAPTCHA (CheckSliderCaptcha thay vì VerifySliderCaptcha) ===
		fmt.Println("\nBước 4: Đang xác thực Captcha...")
		verifyResult, err := cli.CheckSliderCaptcha(xPos)
		if err != nil {
			fmt.Printf("Lỗi khi xác thực captcha: %v\n", err)
			continue
		}
		
		// Kiểm tra kết quả xác thực
		var response CaptchaVerifyResponse
		err = json.Unmarshal([]byte(verifyResult), &response)
		if err != nil {
			fmt.Printf("Lỗi khi parse kết quả xác thực: %v\nDữ liệu: %s\n", err, verifyResult)
			continue
		}
		
		// Kiểm tra nếu có Message (IdyKey)
		if response.Data.Message != "" {
			idyKey = response.Data.Message
			fmt.Printf("- Xác thực thành công! IdyKey: %s\n", idyKey)
			break
		} else {
			fmt.Printf("- Xác thực thất bại! Kết quả: %s\n", verifyResult)
		}
	}
	
	// Kiểm tra nếu không lấy được IdyKey sau nhiều lần thử
	if idyKey == "" {
		fmt.Printf("\nKhông thể xác thực captcha sau %d lần thử. Hủy quá trình đăng nhập.\n", maxAttempts)
		os.Exit(1)
	}

	// Thiết lập IdyKey cho client
	cli.SetIdyKey(idyKey)

	// === BƯỚC 5: ĐĂNG NHẬP (CHỈ KHI ĐÃ CÓ IDYKEY) ===
	fmt.Println("\nBước 5: Đang đăng nhập...")
	loginResult, err := cli.Login()
	if err != nil {
		fmt.Printf("Lỗi khi đăng nhập: %v\n", err)
		os.Exit(1)
	}

	// Hiển thị kết quả đăng nhập thô để debug
	fmt.Printf("- Kết quả đăng nhập thô: %s\n", loginResult)
	
	// Kiểm tra kết quả đăng nhập
	var loginResponse LoginResponse
	err = json.Unmarshal([]byte(loginResult), &loginResponse)
	if err != nil {
		fmt.Printf("Lỗi khi parse kết quả đăng nhập: %v\n", err)
		fmt.Println("Tiếp tục với giả định đăng nhập thành công...")
	} else {
		// Chỉ kiểm tra IsSuccess nếu parse JSON thành công
		if !loginResponse.Data.IsSuccess && loginResponse.Data.Message != "" {
			fmt.Printf("Đăng nhập thất bại: %s\n", loginResponse.Data.Message)
			os.Exit(1)
		}
	}
	
	fmt.Printf("- Đăng nhập thành công!\n")
	
	// === BƯỚC 6: CẬP NHẬT THÔNG TIN SAU ĐĂNG NHẬP ===
	fmt.Println("\nBước 6: Đang cập nhật thông tin sau đăng nhập...")
	err = cli.FetchHomeAfterLogin()
	if err != nil {
		fmt.Printf("Lỗi khi cập nhật thông tin sau đăng nhập: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("- Đã cập nhật RequestVerificationToken mới\n")
	fmt.Printf("- Đã cập nhật Cookies: %s\n", cli.GetAllCookiesFormatted())
	
	// === BƯỚC 7: KIỂM TRA SỐ DƯ ===
	fmt.Println("\nBước 7: Đang kiểm tra số dư tài khoản...")
	balanceResult, err := cli.GetMemberBalance()
	if err != nil {
		fmt.Printf("Lỗi khi kiểm tra số dư: %v\n", err)
		os.Exit(1)
	}
	
	// Hiển thị kết quả thô cho debug
	fmt.Printf("- Kết quả số dư thô: %s\n", balanceResult)
	
	// Phân tích kết quả số dư
	var balanceResponse BalanceResponse
	err = json.Unmarshal([]byte(balanceResult), &balanceResponse)
	if err != nil {
		fmt.Printf("Lỗi khi parse kết quả số dư: %v\nDữ liệu: %s\n", err, balanceResult)
		os.Exit(1)
	}
	
	// Hiển thị thông tin số dư
	fmt.Println("\n=== THÔNG TIN SỐ DƯ ===")
	fmt.Printf("Số dư tài khoản: %d\n", int(balanceResponse.Data.WalletData.BalanceAmount))
	
	// === BƯỚC 8: KIỂM TRA QUYỀN TRUY CẬP LỊCH SỬ GIAO DỊCH ===
	fmt.Println("\nBước 8: Đang kiểm tra quyền truy cập lịch sử giao dịch...")
	accessResult, err := cli.CheckTransactionAccess()
	if err != nil {
		fmt.Printf("Lỗi khi kiểm tra quyền truy cập: %v\n", err)
		os.Exit(1)
	}
	
	// Hiển thị kết quả thô cho debug
	fmt.Printf("- Kết quả kiểm tra quyền truy cập: %s\n", accessResult)
	
	// Phân tích kết quả kiểm tra quyền truy cập
	var accessResponse TransactionAccessResponse
	err = json.Unmarshal([]byte(accessResult), &accessResponse)
	if err != nil {
		fmt.Printf("Lỗi khi parse kết quả quyền truy cập: %v\n", err)
		os.Exit(1)
	}
	
	// Kiểm tra quyền truy cập
	if !accessResponse.Data.IsOpen {
		fmt.Println("- Không có quyền truy cập lịch sử giao dịch!")
		fmt.Println("\n=== HOÀN THÀNH ===")
		os.Exit(0)
	}
	
	fmt.Printf("- Có quyền truy cập lịch sử giao dịch (Giới hạn: %d)\n", accessResponse.Data.LimitCount)
	
	// === BƯỚC 9: LẤY LỊCH SỬ GIAO DỊCH ===
	fmt.Println("\nBước 9: Đang lấy lịch sử giao dịch...")
	historyResult, err := cli.GetTransactionHistory()
	if err != nil {
		fmt.Printf("Lỗi khi lấy lịch sử giao dịch: %v\n", err)
		os.Exit(1)
	}
	
	// Hiển thị kết quả thô cho debug (chỉ phần đầu vì quá dài)
	shortResult := historyResult
	if len(historyResult) > 200 {
		shortResult = historyResult[:200] + "..."
	}
	fmt.Printf("- Kết quả lịch sử giao dịch (đã rút gọn): %s\n", shortResult)
	
	// Phân tích kết quả lịch sử giao dịch
	var historyResponse TransactionHistoryResponse
	err = json.Unmarshal([]byte(historyResult), &historyResponse)
	if err != nil {
		fmt.Printf("Lỗi khi parse kết quả lịch sử giao dịch: %v\n", err)
		os.Exit(1)
	}
	
	// Tìm giao dịch nạp tiền gần nhất (TransType = 1 là nạp tiền)
	var latestDeposit *struct {
		TransactionNumber  string  `json:"TransactionNumber"`
		CreateTime         string  `json:"CreateTime"`
		TransType          int     `json:"TransType"`
		TransContent       int     `json:"TransContent"`
		TransactionAmount  float64 `json:"TransactionAmount"`
		DealType_Sum       int     `json:"DealType_Sum"`
		BalanceAmount      float64 `json:"BalanceAmount"`
		PayNumber          string  `json:"PayNumber"`
		PaywayID           string  `json:"PaywayID"`
	}
	
	for _, transaction := range historyResponse.Data.Data {
		// Kiểm tra nếu là giao dịch nạp tiền (TransType = 1)
		if transaction.TransType == 1 {
			latestDeposit = &transaction
			break
		}
	}
	
	// Hiển thị thông tin giao dịch nạp tiền gần nhất
	fmt.Println("\n=== GIAO DỊCH NẠP TIỀN GẦN NHẤT ===")
	if latestDeposit != nil {
		// Chuyển đổi thời gian từ UTC sang múi giờ HCM (+7)
		timeStr := latestDeposit.CreateTime
		// Loại bỏ phần mili giây nếu có
		timeStr = strings.Split(timeStr, ".")[0]
		// Parse thời gian
		t, err := time.Parse("2006-01-02T15:04:05", timeStr)
		if err != nil {
			fmt.Printf("Lỗi khi parse thời gian: %v\n", err)
			// Sử dụng thời gian gốc nếu lỗi
			fmt.Printf("Thời gian: %s\n", latestDeposit.CreateTime)
		} else {
			// Chuyển đổi sang múi giờ HCM (+7)
			t = t.Add(7 * time.Hour)
			fmt.Printf("Thời gian: %s\n", t.Format("02/01/2006 15:04:05"))
		}
		
		fmt.Printf("Số tiền: %d\n", int(latestDeposit.TransactionAmount))
		fmt.Printf("Số dư sau giao dịch: %d\n", int(latestDeposit.BalanceAmount))
		fmt.Printf("Mã giao dịch: %s\n", latestDeposit.TransactionNumber)
		fmt.Printf("Phương thức: %s\n", latestDeposit.PaywayID)
	} else {
		fmt.Println("Không tìm thấy giao dịch nạp tiền nào!")
	}

	fmt.Println("\n=== HOÀN THÀNH ===")
} 