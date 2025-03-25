package client

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bongg/autologin/config"
	"github.com/bongg/autologin/utils"
	"github.com/go-resty/resty/v2"
)

// Client là HTTP client để tương tác với API
type Client struct {
	client     *resty.Client
	config     *config.Config
	token      string
	idyKey     string
	cookie     string
	cookieAll  string // Cookie đầy đủ cho request
	fingerIDX  string
	allCookies string
	userAgent  string // Thêm trường lưu User-Agent
}

// NewClient tạo một HTTP client mới
func NewClient(cfg *config.Config) *Client {
	client := resty.New()
	client.SetCookieJar(nil) // Tự động xử lý cookies

	// Thiết lập proxy nếu có
	if cfg.ProxyURL != "" {
		client.SetProxy(cfg.ProxyURL)
	}

	// Xác định User-Agent và đặt cho toàn bộ client
	userAgent := cfg.UserAgent
	client.SetHeader("User-Agent", userAgent)

	return &Client{
		client:    client,
		config:    cfg,
		fingerIDX: utils.GenerateFingerIDX(),
		userAgent: userAgent, // Lưu lại User-Agent để tái sử dụng
	}
}

// FetchInitialData lấy token và cookies từ trang chủ
func (c *Client) FetchInitialData() error {
	// Thiết lập timeout để tránh blocking
	c.client.SetTimeout(30 * time.Second)

	// GET trang chủ - đảm bảo sử dụng User-Agent đã lưu
	c.client.SetHeader("User-Agent", c.userAgent) // Đảm bảo User-Agent đúng
	resp, err := c.client.R().
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8").
		Get(c.config.BaseURL + "/Home/Index")

	if err != nil {
		return fmt.Errorf("lỗi truy cập trang chủ: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("status code không hợp lệ: %d", resp.StatusCode())
	}

	html := string(resp.Body())

	// Lưu lại tất cả headers và cookies
	c.allCookies = ""
	for k, v := range resp.Header() {
		if k == "Set-Cookie" {
			for _, cookie := range v {
				c.allCookies += cookie + "\n"
			}
		}
	}

	// Trích xuất token
	c.token = utils.ExtractToken(html)
	if c.token == "" {
		return fmt.Errorf("không tìm thấy token")
	}

	// Trích xuất IdyKey - không bắt buộc
	c.idyKey = utils.ExtractIdyKey(html)

	// Trích xuất cookie IT từ cookies - không bắt buộc nữa
	cookies := c.allCookies
	c.cookie = utils.ExtractCookie(cookies)

	// Trích xuất toàn bộ cookie cần thiết
	c.cookieAll = utils.ExtractAllCookies(cookies)

	// Tạo FingerIDX giả
	c.fingerIDX = utils.GenerateFingerIDX()

	return nil
}

// GetToken trả về token đã lấy được
func (c *Client) GetToken() string {
	return c.token
}

// GetIdyKey trả về IdyKey đã lấy được
func (c *Client) GetIdyKey() string {
	return c.idyKey
}

// GetCookie trả về cookie IT đã lấy được
func (c *Client) GetCookie() string {
	return c.cookie
}

// GetFingerIDX trả về FingerIDX
func (c *Client) GetFingerIDX() string {
	return c.fingerIDX
}

// GetAllCookies trả về tất cả cookies đã lấy được
func (c *Client) GetAllCookies() string {
	return c.allCookies
}

// GetUserAgent trả về User-Agent đang sử dụng
func (c *Client) GetUserAgent() string {
	return c.userAgent
}

// GetAllCookiesFormatted trả về tất cả cookies đã được định dạng
func (c *Client) GetAllCookiesFormatted() string {
	return c.cookieAll
}

// Login thực hiện đăng nhập
func (c *Client) Login() (string, error) {
	// Thiết lập timeout để tránh blocking
	c.client.SetTimeout(30 * time.Second)

	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// Tạo body request
	reqBody := map[string]interface{}{
		"AccountID":            c.config.Username,
		"AccountPWD":           utils.EncodePassword(c.config.Password),
		"ProtectCode":          "",
		"LocalStorgeCookie":    c.cookie,
		"FingerIDX":            c.fingerIDX,
		"ScreenResolution":     "1920*1080",
		"ShowSliderCaptcha":    true,
		"ShowPhoneVerify":      false,
		"VerifySliderCaptcha":  true,
		"CellPhone":            "",
		"ProtectCodeCellPhone": "",
		"IsCellPhoneValid":     false,
		"IdyKey":               c.idyKey,
		"CaptchaCode":          "",
		"LoginVerification":    1,
		"IsLobbyProtect":       false,
		"UniqueSessionId":      "TM" + utils.GetTimestamp(),
	}

	// Gửi request đăng nhập
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json;charset=UTF-8").
		SetHeader("requestverificationtoken", c.token).
		SetHeader("referer", c.config.BaseURL+"/Home/Index").
		SetHeader("origin", c.config.BaseURL).
		SetHeader("x-requested-with", "XMLHttpRequest").
		SetHeader("uniquetick", utils.GetTimestamp()).
		SetBody(reqBody).
		Post(c.config.LoginURL)

	if err != nil {
		return "", fmt.Errorf("lỗi khi gửi request đăng nhập: %v", err)
	}

	// Lưu lại tất cả cookies từ response đăng nhập
	c.allCookies = ""
	for k, v := range resp.Header() {
		if k == "Set-Cookie" {
			for _, cookie := range v {
				c.allCookies += cookie + "\n"
			}
		}
	}

	// Cập nhật cookie nếu có cookies mới
	if c.allCookies != "" {
		newCookies := utils.ExtractAllCookies(c.allCookies)
		if newCookies != "" {
			c.cookieAll = newCookies
		}
	}

	// Phân tích JSON response để lấy CookieID
	bodyStr := string(resp.Body())

	// Tạo map để parse JSON
	var response map[string]interface{}
	err = json.Unmarshal([]byte(bodyStr), &response)
	if err == nil {
		// Trích xuất CookieID từ JSON response
		if data, ok := response["Data"].(map[string]interface{}); ok {
			if cookieID, ok := data["CookieID"].(string); ok && cookieID != "" {
				// Nếu đã có cookies, thêm cookieID vào
				if c.cookieAll != "" {
					c.cookieAll += "; BBOSID=" + cookieID
				} else {
					c.cookieAll = "_culture=vi-vn; BBOSID=" + cookieID
				}

				// Tạo thêm BBOAUTH từ cookieID
				c.cookieAll += "; BBOAUTH=" + cookieID + "X" + utils.GetTimestamp()

				// Thêm cookieID vào cookies hiện tại nếu chưa có
				if !strings.Contains(c.cookieAll, "CookieID=") {
					c.cookieAll += "; CookieID=" + cookieID
				}
			}
		}
	}

	return bodyStr, nil
}

// GetSliderCaptcha lấy thông tin captcha từ API
func (c *Client) GetSliderCaptcha() (string, error) {
	// Thiết lập timeout để tránh blocking
	c.client.SetTimeout(30 * time.Second)

	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// Sử dụng lại client với cùng User-Agent để đảm bảo phiên nhất quán
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("requestverificationtoken", c.token).
		SetHeader("referer", c.config.BaseURL+"/Home/Index").
		SetHeader("origin", c.config.BaseURL).
		SetHeader("x-requested-with", "XMLHttpRequest").
		SetHeader("uniquetick", utils.GetTimestamp()).
		Post(c.config.BaseURL + "/api/Verify/GetSliderCaptcha")

	if err != nil {
		return "", fmt.Errorf("lỗi khi gửi request lấy captcha: %v", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("status code không hợp lệ: %d", resp.StatusCode())
	}

	return string(resp.Body()), nil
}

// VerifySliderCaptcha gửi kết quả captcha để xác thực
func (c *Client) VerifySliderCaptcha(xPos int) (string, error) {
	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// Thêm timeout cho request để tránh blocking
	c.client.SetTimeout(30 * time.Second)

	// Gửi request với vị trí X của thanh trượt
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("requestverificationtoken", c.token).
		SetHeader("referer", c.config.BaseURL+"/Home/Index").
		SetHeader("origin", c.config.BaseURL).
		SetHeader("x-requested-with", "XMLHttpRequest").
		SetHeader("uniquetick", utils.GetTimestamp()).
		SetBody(map[string]interface{}{
			"XPosition": xPos,
		}).
		Post(c.config.BaseURL + "/api/Verify/VerifySliderCaptcha")

	if err != nil {
		return "", fmt.Errorf("lỗi khi gửi request xác thực: %v", err)
	}

	return string(resp.Body()), nil
}

// CheckSliderCaptcha kiểm tra slider captcha với vị trí x
func (c *Client) CheckSliderCaptcha(xPos int) (string, error) {
	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// Thêm timeout cho request để tránh blocking
	c.client.SetTimeout(30 * time.Second)

	// Tạo giả mảng Trail - mô phỏng quá trình kéo trượt
	trail := make([]int, 100)

	// Điểm bắt đầu mặc định
	trail[0] = xPos
	trail[1] = 0 // Y luôn = 0

	// Giả lập quá trình kéo từ 0 đến xPos
	for i := 2; i < 100; i++ {
		if i%2 == 0 {
			// Tính toán vị trí X tăng dần
			step := i / 2
			if step < xPos {
				trail[i] = step
			} else {
				trail[i] = xPos
			}
		} else {
			// Y luôn = 0
			trail[i] = 0
		}
	}

	// POST để xác thực
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json;charset=UTF-8").
		SetHeader("requestverificationtoken", c.token).
		SetHeader("referer", c.config.BaseURL+"/Home/Index").
		SetHeader("origin", c.config.BaseURL).
		SetHeader("x-requested-with", "XMLHttpRequest").
		SetHeader("uniquetick", utils.GetTimestamp()).
		SetBody(map[string]interface{}{
			"Trail": trail,
		}).
		Post(c.config.BaseURL + "/api/Verify/CheckSliderCaptcha")

	if err != nil {
		return "", fmt.Errorf("lỗi khi gửi request xác thực: %v", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("status code không hợp lệ: %d", resp.StatusCode())
	}

	return string(resp.Body()), nil
}

// SetIdyKey thiết lập giá trị IdyKey cho client
func (c *Client) SetIdyKey(idyKey string) {
	c.idyKey = idyKey
}

// FetchHomeAfterLogin lấy thông tin mới từ trang chủ sau khi đăng nhập
func (c *Client) FetchHomeAfterLogin() error {
	// Lưu lại cookies quan trọng trước khi cập nhật
	originalBBOSID := ""
	originalBBOAUTH := ""

	// Trích xuất BBOSID hiện tại
	bbosidRegex := regexp.MustCompile(`BBOSID=([^;]+)`)
	if matches := bbosidRegex.FindStringSubmatch(c.cookieAll); len(matches) > 1 {
		originalBBOSID = matches[1]
	}

	// Trích xuất BBOAUTH hiện tại
	bboauthRegex := regexp.MustCompile(`BBOAUTH=([^;]+)`)
	if matches := bboauthRegex.FindStringSubmatch(c.cookieAll); len(matches) > 1 {
		originalBBOAUTH = matches[1]
	}

	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// GET trang chủ sau khi đăng nhập
	resp, err := c.client.R().
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8").
		SetHeader("Cookie", c.cookieAll). // Đảm bảo gửi cookie đúng
		Get(c.config.BaseURL + "/Home/Index")

	if err != nil {
		return fmt.Errorf("lỗi truy cập trang chủ sau đăng nhập: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("status code không hợp lệ: %d", resp.StatusCode())
	}

	html := string(resp.Body())

	// Lưu lại tất cả headers và cookies
	c.allCookies = ""
	for k, v := range resp.Header() {
		if k == "Set-Cookie" {
			for _, cookie := range v {
				c.allCookies += cookie + "\n"
			}
		}
	}

	// Trích xuất token mới
	newToken := utils.ExtractToken(html)
	if newToken != "" {
		c.token = newToken
	}

	// Cập nhật cookie sau đăng nhập
	newCookies := utils.ExtractAllCookies(c.allCookies)

	// Nếu mất BBOSID hoặc BBOAUTH, thêm lại vào
	hasBBOSID := regexp.MustCompile(`BBOSID=`).MatchString(newCookies)
	hasBBOAUTH := regexp.MustCompile(`BBOAUTH=`).MatchString(newCookies)

	if !hasBBOSID && originalBBOSID != "" {
		if newCookies != "" {
			newCookies += "; "
		}
		newCookies += "BBOSID=" + originalBBOSID
	}

	if !hasBBOAUTH && originalBBOAUTH != "" {
		if newCookies != "" {
			newCookies += "; "
		}
		newCookies += "BBOAUTH=" + originalBBOAUTH
	}

	c.cookieAll = newCookies
	// fmt.Printf("Updated cookies: %s\n", c.cookieAll)

	return nil
}

// GetMemberBalance lấy thông tin số dư tài khoản
func (c *Client) GetMemberBalance() (string, error) {
	// Endpoint cho kiểm tra số dư
	endpoint := c.config.BaseURL + "/api/MemberTransfer/GetMemberBalanceInfoByAccountID"
	fmt.Printf("Checking balance with endpoint: %s\n", endpoint)

	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// Gửi request kiểm tra số dư với đầy đủ headers
	req := c.client.R().
		SetHeader("Content-Type", "application/json;charset=UTF-8").
		SetHeader("requestverificationtoken", c.token).
		SetHeader("referer", c.config.BaseURL+"/Home/Index").
		SetHeader("origin", c.config.BaseURL).
		SetHeader("x-requested-with", "XMLHttpRequest").
		SetHeader("accept", "application/json, text/plain, */*").
		SetHeader("accept-language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7").
		SetHeader("uniquetick", utils.GetTimestamp()).
		SetBody("{}") // Thêm body rỗng để tránh lỗi 411 Length Required

	// Đặt cookie đầy đủ
	if c.cookieAll != "" {
		req.SetHeader("Cookie", c.cookieAll)
	}

	// Thực hiện request
	resp, err := req.Post(endpoint)

	if err != nil {
		// Thử lại một lần nếu request thất bại
		time.Sleep(1 * time.Second)
		resp, err = req.Post(endpoint)

		if err != nil {
			return "", fmt.Errorf("lỗi kiểm tra số dư (sau khi thử lại): %v", err)
		}
	}

	// Hiển thị status code để debug
	fmt.Printf("Status code from GetMemberBalance: %d\n", resp.StatusCode())

	if resp.StatusCode() != 200 {
		if resp.StatusCode() == 403 {
			return "", fmt.Errorf("lỗi 403 Forbidden - cookie không hợp lệ hoặc đã hết hạn")
		}
		return "", fmt.Errorf("status code không hợp lệ: %d", resp.StatusCode())
	}

	return string(resp.Body()), nil
}

// CheckTransactionAccess kiểm tra quyền truy cập vào lịch sử giao dịch
func (c *Client) CheckTransactionAccess() (string, error) {
	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// Gửi request kiểm tra quyền truy cập
	req := c.client.R().
		SetHeader("Content-Type", "application/json;charset=UTF-8").
		SetHeader("requestverificationtoken", c.token).
		SetHeader("referer", c.config.BaseURL+"/Member/TransactionRecords").
		SetHeader("origin", c.config.BaseURL).
		SetHeader("x-requested-with", "XMLHttpRequest").
		SetHeader("accept", "application/json, text/plain, */*").
		SetHeader("accept-language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7").
		SetHeader("uniquetick", utils.GetTimestamp())

	// Đặt cookie nếu có
	if c.cookieAll != "" {
		req.SetHeader("Cookie", c.cookieAll)
	}

	// Thực hiện request
	resp, err := req.Post(c.config.BaseURL + "/api/Common/GetTransactionRecordUploadSetting")

	if err != nil {
		return "", fmt.Errorf("lỗi khi kiểm tra quyền truy cập: %v", err)
	}

	// Debug status code
	fmt.Printf("Status code from CheckTransactionAccess: %d\n", resp.StatusCode())

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("status code không hợp lệ: %d", resp.StatusCode())
	}

	return string(resp.Body()), nil
}

// GetTransactionHistory lấy lịch sử giao dịch
func (c *Client) GetTransactionHistory() (string, error) {
	// Đảm bảo User-Agent nhất quán
	c.client.SetHeader("User-Agent", c.userAgent)

	// Tạo body request
	reqBody := map[string]interface{}{
		"TransType":    0,
		"QueryType":    1,
		"PageNumber":   0,
		"RecordCounts": 10,
		"OrderField":   "CreateTime",
		"Desc":         "true",
	}

	// Gửi request lấy lịch sử giao dịch
	req := c.client.R().
		SetHeader("Content-Type", "application/json;charset=UTF-8").
		SetHeader("requestverificationtoken", c.token).
		SetHeader("referer", c.config.BaseURL+"/Member/TransactionRecords").
		SetHeader("origin", c.config.BaseURL).
		SetHeader("x-requested-with", "XMLHttpRequest").
		SetHeader("accept", "application/json, text/plain, */*").
		SetHeader("accept-language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7").
		SetHeader("uniquetick", utils.GetTimestamp()).
		SetBody(reqBody)

	// Đặt cookie nếu có
	if c.cookieAll != "" {
		req.SetHeader("Cookie", c.cookieAll)
	}

	// Thực hiện request
	resp, err := req.Post(c.config.BaseURL + "/api/TransactionRecords/GetMemberWalletSumLogByCondition")

	if err != nil {
		return "", fmt.Errorf("lỗi khi lấy lịch sử giao dịch: %v", err)
	}

	// Debug status code
	fmt.Printf("Status code from GetTransactionHistory: %d\n", resp.StatusCode())

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("status code không hợp lệ: %d", resp.StatusCode())
	}

	return string(resp.Body()), nil
}
