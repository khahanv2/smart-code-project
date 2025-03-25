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
)

// CaptchaVerifyResponse cấu trúc phản hồi từ API verify captcha
type CaptchaVerifyResponse struct {
	Data struct {
		Message string `json:"Message"`
	} `json:"Data"`
}

func main() {
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
			fmt.Printf("Tọa độ X không hợp lệ: %v\n", err)
			os.Exit(1)
		}
	}

	// === BƯỚC 1: LẤY THÔNG TIN BAN ĐẦU ===
	fmt.Println("=== LẤY THÔNG TIN TỪ TRANG CHỦ ===")
	err = cli.FetchInitialData()
	if err != nil {
		fmt.Printf("Lỗi khi lấy dữ liệu ban đầu: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("User-Agent:")
	fmt.Printf("%s\n\n", cli.GetUserAgent())
	
	fmt.Println("RequestVerificationToken:")
	fmt.Printf("%s\n\n", cli.GetToken())

	fmt.Println("Cookie:")
	fmt.Printf("%s\n\n", cli.GetCookie())
	
	fmt.Println("FingerIDX:")
	fmt.Printf("%s\n", cli.GetFingerIDX())

	// === LẤY VÀ GIẢI CAPTCHA TRONG VÒNG LẶP CHO ĐẾN KHI THÀNH CÔNG ===
	var idyKey string
	maxAttempts := 5
	attempt := 0

	for attempt < maxAttempts {
		attempt++
		fmt.Printf("\n=== LẦN THỬ %d: LẤY SLIDER CAPTCHA ===\n", attempt)
		
		// Lấy captcha JSON
		captchaJSON, err := cli.GetSliderCaptcha()
		if err != nil {
			fmt.Printf("Lỗi khi lấy captcha: %v\n", err)
			continue
		}
		
		// Lưu JSON captcha vào file
		fileName := fmt.Sprintf("captcha_%d.json", time.Now().Unix())
		err = os.WriteFile(fileName, []byte(captchaJSON), 0644)
		if err != nil {
			fmt.Printf("Lỗi khi lưu file captcha: %v\n", err)
		} else {
			fmt.Printf("Đã lưu dữ liệu captcha vào file: %s\n\n", fileName)
		}

		// Xác định tọa độ X
		var xPos int
		if providedX > 0 {
			// Sử dụng tọa độ X được cung cấp từ tham số dòng lệnh
			xPos = providedX
			fmt.Printf("Sử dụng tọa độ X được chỉ định: %d\n", xPos)
		} else {
			// Giải captcha để lấy tọa độ X
			fmt.Println("=== GIẢI CAPTCHA ===")
			startTime := time.Now()
			xPos, err = captcha.SolveCaptcha(captchaJSON)
			if err != nil {
				fmt.Printf("Lỗi khi giải captcha: %v\n", err)
				continue
			}
			elapsedTime := time.Since(startTime)
			fmt.Printf("Vị trí X: %d (giải trong %.2f giây)\n", xPos, elapsedTime.Seconds())
		}

		// === XÁC THỰC CAPTCHA ===
		fmt.Println("\n=== XÁC THỰC CAPTCHA ===")
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
			fmt.Printf("Xác thực thành công!\nIdyKey: %s\n", idyKey)
			break
		} else {
			fmt.Printf("Xác thực thất bại! Kết quả: %s\n", verifyResult)
			// Nếu dùng tọa độ X được chỉ định từ tham số và thất bại, thì thử giải tự động
			if providedX > 0 {
				providedX = 0
				fmt.Println("Chuyển sang chế độ giải tự động cho lần thử tiếp theo")
			}
		}
	}

	// Hiển thị thông tin cuối cùng
	fmt.Println("\n=== THÔNG TIN CHO CURL ===")
	fmt.Printf("-H 'user-agent: %s'\n", cli.GetUserAgent())
	fmt.Printf("-H 'requestverificationtoken: %s'\n", cli.GetToken())
	fmt.Printf("-b 'IT=%s'\n", cli.GetCookie())
	
	if idyKey != "" {
		fmt.Printf("\n=== THÔNG TIN ĐĂNG NHẬP ===\n")
		fmt.Printf("IdyKey: %s\n", idyKey)
		fmt.Printf("FingerIDX: %s\n", cli.GetFingerIDX())
		fmt.Printf("LocalStorgeCookie: %s\n", cli.GetCookie())
	} else {
		fmt.Println("\nKhông thể xác thực captcha sau nhiều lần thử!")
	}
} 