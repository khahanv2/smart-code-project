package utils

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Danh sách User-Agent phổ biến và thực tế
var userAgents = []string{
	// Chrome - Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
	
	// Chrome - Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	
	// Chrome - Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	
	// Firefox
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/118.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/115.0",
	
	// Safari
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.3 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	
	// Edge
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0",
	
	// Mobile - iOS
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",
	
	// Mobile - Android
	"Mozilla/5.0 (Linux; Android 13; SM-S901B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
}

// GenerateRandomUserAgent trả về một User-Agent ngẫu nhiên từ danh sách thực tế
func GenerateRandomUserAgent() string {
	// Khởi tạo random seed
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	// Chọn ngẫu nhiên từ danh sách
	return userAgents[r.Intn(len(userAgents))]
}

// EncodePassword mã hóa password dưới dạng base64
func EncodePassword(password string) string {
	return base64.StdEncoding.EncodeToString([]byte(password))
}

// GetTimestamp lấy timestamp hiện tại dưới dạng milliseconds
func GetTimestamp() string {
	return strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
}

// ExtractToken trích xuất token từ HTML
func ExtractToken(html string) string {
	tokenRegex := regexp.MustCompile(`<ajax-anti-forgery-token token="([^"]+)"></ajax-anti-forgery-token>`)
	matches := tokenRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractIdyKey trích xuất IdyKey từ HTML
func ExtractIdyKey(html string) string {
	idyKeyRegex := regexp.MustCompile(`IdyKey['"]*:\s*['"]([\w-]+)['"]`)
	matches := idyKeyRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractCookie trích xuất giá trị cookie IT từ cookies
func ExtractCookie(cookieStr string) string {
	itRegex := regexp.MustCompile(`IT=([^;]+)`)
	matches := itRegex.FindStringSubmatch(cookieStr)
	if len(matches) > 1 {
		return matches[1]
	}
	
	// Thử tìm BBOSID nếu không có IT
	bbosidRegex := regexp.MustCompile(`BBOSID=([^;]+)`)
	matches = bbosidRegex.FindStringSubmatch(cookieStr)
	if len(matches) > 1 {
		return matches[1]
	}
	
	return ""
}

// GenerateFingerIDX tạo giả lập fingerprint ID
func GenerateFingerIDX() string {
	// Trong thực tế, đây sẽ được tính toán phức tạp hơn
	return fmt.Sprintf("web_fingerprint_%d", time.Now().Unix())
}

// ExtractAllCookies trích xuất tất cả cookies cần thiết từ header response
func ExtractAllCookies(cookieStr string) string {
	// Tạo string cookie theo định dạng của curl
	var cookieParts []string
	
	// Trích xuất _culture
	cultureRegex := regexp.MustCompile(`_culture=([^;]+)`)
	if matches := cultureRegex.FindStringSubmatch(cookieStr); len(matches) > 1 {
		cookieParts = append(cookieParts, "_culture="+matches[1])
	} else {
		cookieParts = append(cookieParts, "_culture=vi-vn")
	}
	
	// Trích xuất IT
	itRegex := regexp.MustCompile(`IT=([^;]+)`)
	if matches := itRegex.FindStringSubmatch(cookieStr); len(matches) > 1 {
		cookieParts = append(cookieParts, "IT="+matches[1])
	}
	
	// Trích xuất BBOSID
	bbosidRegex := regexp.MustCompile(`BBOSID=([^;]+)`)
	if matches := bbosidRegex.FindStringSubmatch(cookieStr); len(matches) > 1 {
		cookieParts = append(cookieParts, "BBOSID="+matches[1])
	}
	
	// Trích xuất targetUrl
	targetUrlRegex := regexp.MustCompile(`targetUrl=([^;]+)`)
	if matches := targetUrlRegex.FindStringSubmatch(cookieStr); len(matches) > 1 {
		cookieParts = append(cookieParts, "targetUrl="+matches[1])
	}
	
	// Trích xuất BBOAUTH
	bboauthRegex := regexp.MustCompile(`BBOAUTH=([^;]+)`)
	if matches := bboauthRegex.FindStringSubmatch(cookieStr); len(matches) > 1 {
		cookieParts = append(cookieParts, "BBOAUTH="+matches[1])
	}
	
	// Nối các phần cookie lại với nhau
	return strings.Join(cookieParts, "; ")
} 