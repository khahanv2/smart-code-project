package main

import (
	"fmt"
	"os"

	"github.com/bongg/autologin/client"
	"github.com/bongg/autologin/config"
	"github.com/bongg/autologin/utils"
)

func main() {
	// Tạo cấu hình với giá trị mặc định
	cfg := config.NewConfig("", "")

	// Tạo client
	cli := client.NewClient(cfg)

	// Lấy dữ liệu ban đầu (token, cookies)
	fmt.Println("Đang lấy thông tin từ trang chủ...")
	err := cli.FetchInitialData()
	if err != nil {
		fmt.Printf("Lỗi khi lấy dữ liệu ban đầu: %v\n", err)
		os.Exit(1)
	}

	// Hiển thị các thông tin đã lấy được
	fmt.Println("\n=== THÔNG TIN ĐÃ LẤY ĐƯỢC ===")

	fmt.Println("User-Agent:")
	fmt.Printf("%s\n\n", cli.GetUserAgent())

	fmt.Println("RequestVerificationToken:")
	fmt.Printf("%s\n\n", cli.GetToken())

	cookieValue := cli.GetCookie()
	cookieType := "BBOSID"
	if utils.ExtractCookie(fmt.Sprintf("IT=%s", cookieValue)) != "" {
		cookieType = "IT"
	}

	fmt.Printf("Cookie %s:\n", cookieType)
	fmt.Printf("%s\n\n", cookieValue)

	fmt.Println("FingerIDX (Giả lập):")
	fmt.Printf("%s\n\n", cli.GetFingerIDX())

	fmt.Println("Tất cả cookies:")
	fmt.Printf("%s\n", cli.GetAllCookies())

	if idyKey := cli.GetIdyKey(); idyKey != "" {
		fmt.Println("\nIdyKey (nếu có):")
		fmt.Printf("%s\n", idyKey)
	}

	// Lấy thông tin Slider Captcha (giữ nguyên phiên)
	fmt.Println("\n=== LẤY SLIDER CAPTCHA ===")
	captchaData, err := cli.GetSliderCaptcha()
	if err != nil {
		fmt.Printf("Lỗi khi lấy captcha: %v\n", err)
	} else {
		fmt.Println("Dữ liệu Captcha (JSON):")
		fmt.Println(captchaData)
	}

	fmt.Println("\n=== THÔNG TIN CHO CURL ===")
	fmt.Printf("-H 'user-agent: %s'\n", cli.GetUserAgent())
	fmt.Printf("-H 'requestverificationtoken: %s'\n", cli.GetToken())
	if cli.GetCookie() != "" {
		cookieValue := cli.GetCookie()
		cookieType := "BBOSID"
		if utils.ExtractCookie(fmt.Sprintf("IT=%s", cookieValue)) != "" {
			cookieType = "IT"
		}
		fmt.Printf("-b '%s=%s'\n", cookieType, cookieValue)
	}
}
