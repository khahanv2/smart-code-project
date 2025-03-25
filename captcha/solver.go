package captcha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CaptchaData chứa dữ liệu từ API captcha
type CaptchaData struct {
	Data struct {
		Slider     string `json:"Slider"`
		Background string `json:"Background"`
	} `json:"Data"`
}

// CaptchaResult chứa kết quả xử lý captcha
type CaptchaResult struct {
	X int `json:"x"`
}

// SolveCaptcha gửi dữ liệu captcha cho captcha_solver và trả về tọa độ X
func SolveCaptcha(captchaJSON string) (int, error) {
	// Kiểm tra dữ liệu JSON hợp lệ
	var captchaData CaptchaData
	err := json.Unmarshal([]byte(captchaJSON), &captchaData)
	if err != nil {
		return 0, fmt.Errorf("dữ liệu JSON không hợp lệ: %v", err)
	}

	// Đảm bảo rằng có dữ liệu slider và background
	if captchaData.Data.Slider == "" || captchaData.Data.Background == "" {
		return 0, fmt.Errorf("thiếu dữ liệu slider hoặc background")
	}

	// Sử dụng pipe để giao tiếp với captcha_solver
	cmd := exec.Command("./captcha_solver", "--pipe")
	
	// Tạo stdin pipe để gửi dữ liệu JSON
	cmd.Stdin = strings.NewReader(captchaJSON)
	
	// Tạo buffer để nhận kết quả
	var out bytes.Buffer
	cmd.Stdout = &out
	
	// Chạy lệnh
	err = cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("lỗi khi chạy captcha_solver: %v", err)
	}
	
	// Xử lý kết quả
	result := strings.TrimSpace(out.String())
	
	// Nếu kết quả là JSON, parse như đối tượng
	if strings.HasPrefix(result, "{") {
		var captchaResult CaptchaResult
		err = json.Unmarshal([]byte(result), &captchaResult)
		if err != nil {
			return 0, fmt.Errorf("lỗi khi parse kết quả JSON: %v", err)
		}
		return captchaResult.X, nil
	}
	
	// Nếu kết quả là số đơn giản, chuyển đổi thành int
	x, err := strconv.Atoi(result)
	if err != nil {
		return 0, fmt.Errorf("kết quả không phải là số hợp lệ: %v", err)
	}
	
	return x, nil
}

// SolveCaptchaSocket sử dụng socket để kết nối tới captcha_solver service
func SolveCaptchaSocket(captchaJSON, serverAddr string, port int) (int, error) {
	// Kiểm tra dữ liệu JSON hợp lệ
	var captchaData CaptchaData
	err := json.Unmarshal([]byte(captchaJSON), &captchaData)
	if err != nil {
		return 0, fmt.Errorf("dữ liệu JSON không hợp lệ: %v", err)
	}

	// Đảm bảo rằng có dữ liệu slider và background
	if captchaData.Data.Slider == "" || captchaData.Data.Background == "" {
		return 0, fmt.Errorf("thiếu dữ liệu slider hoặc background")
	}

	// Tạo kết nối đến service
	addr := fmt.Sprintf("%s:%d", serverAddr, port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return 0, fmt.Errorf("không thể kết nối đến captcha service: %v", err)
	}
	defer conn.Close()

	// Thêm ký tự xuống dòng để service biết kết thúc input
	captchaJSON = captchaJSON + "\n"

	// Gửi dữ liệu JSON
	_, err = conn.Write([]byte(captchaJSON))
	if err != nil {
		return 0, fmt.Errorf("lỗi khi gửi dữ liệu: %v", err)
	}

	// Nhận kết quả
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, fmt.Errorf("lỗi khi nhận kết quả: %v", err)
	}

	// Xử lý kết quả
	result := strings.TrimSpace(string(buf[:n]))

	// Nếu kết quả là JSON, parse như đối tượng
	if strings.HasPrefix(result, "{") {
		var captchaResult CaptchaResult
		err = json.Unmarshal([]byte(result), &captchaResult)
		if err != nil {
			return 0, fmt.Errorf("lỗi khi parse kết quả JSON: %v", err)
		}
		return captchaResult.X, nil
	}

	// Nếu kết quả là số đơn giản, chuyển đổi thành int
	x, err := strconv.Atoi(result)
	if err != nil {
		return 0, fmt.Errorf("kết quả không phải là số hợp lệ: %v", err)
	}

	return x, nil
} 