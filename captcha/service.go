package captcha

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"net"
)

var (
	serviceRunning   bool
	serviceProcess   *os.Process
	servicePort      int = 9876 // Cổng mặc định
	serviceAddr      string = "localhost"
	serviceFailure   bool       // Nếu true, sẽ sử dụng pipe thay vì service
)

// StartCaptchaService khởi động captcha_solver service
func StartCaptchaService(port int) error {
	// Kiểm tra nếu service đã đang chạy
	if serviceRunning && serviceProcess != nil {
		// Kiểm tra xem process có thực sự còn chạy không
		err := serviceProcess.Signal(syscall.Signal(0))
		if err == nil {
			// Process vẫn đang chạy, không cần khởi động lại
			return nil
		}
		// Process không còn chạy, cập nhật trạng thái
		serviceRunning = false
		serviceProcess = nil
	}

	// Kiểm tra kết nối trước khi khởi động service mới
	if isServiceRunning(serviceAddr, port) {
		fmt.Printf("Captcha service đã đang chạy trên cổng %d\n", port)
		serviceRunning = true
		servicePort = port
		return nil
	}

	fmt.Printf("Khởi động captcha service trên cổng %d...\n", port)

	// Khởi động service
	cmd := exec.Command("./captcha_solver", "--service", "--port", strconv.Itoa(port))
	// Chạy trong background
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("không thể khởi động captcha service: %v", err)
	}

	// Lưu process ID
	serviceProcess = cmd.Process
	serviceRunning = true
	servicePort = port

	// Đợi service khởi động
	for i := 0; i < 10; i++ {
		if isServiceRunning(serviceAddr, port) {
			fmt.Printf("Captcha service đã khởi động thành công trên cổng %d\n", port)
			return nil
		}
		fmt.Printf("Đang đợi service khởi động (thử %d/10)...\n", i+1)
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("đã khởi động service nhưng không thể kết nối")
}

// StopCaptchaService dừng captcha service
func StopCaptchaService() {
	if serviceRunning && serviceProcess != nil {
		fmt.Println("Dừng captcha service...")
		err := serviceProcess.Kill()
		if err != nil {
			log.Printf("Lỗi khi dừng captcha service: %v", err)
		} else {
			fmt.Println("Đã dừng captcha service")
		}
		serviceRunning = false
		serviceProcess = nil
	}
}

// isServiceRunning kiểm tra xem service có đang chạy trên port chỉ định không
func isServiceRunning(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetServiceInfo trả về thông tin về service
func GetServiceInfo() (bool, int, string) {
	return serviceRunning, servicePort, serviceAddr
}

// SolveCaptchaWithService giải captcha bằng cách sử dụng service
func SolveCaptchaWithService(captchaJSON string) (int, error) {
	// Nếu đã biết service thất bại trước đó, sử dụng pipe trực tiếp
	if serviceFailure {
		return SolveCaptcha(captchaJSON)
	}

	// Đảm bảo service đang chạy
	if !serviceRunning {
		err := StartCaptchaService(servicePort)
		if err != nil {
			// Ghi nhận việc service thất bại và chuyển sang dùng pipe
			serviceFailure = true
			fmt.Println("Chuyển sang sử dụng pipe do service không khởi động được")
			return SolveCaptcha(captchaJSON)
		}
	}

	// Thử sử dụng socket connection để giải captcha
	result, err := SolveCaptchaSocket(captchaJSON, serviceAddr, servicePort)
	if err != nil {
		// Nếu lỗi kết nối, đánh dấu service thất bại và dùng pipe
		serviceFailure = true
		fmt.Printf("Không thể kết nối đến service: %v, chuyển sang sử dụng pipe\n", err)
		return SolveCaptcha(captchaJSON)
	}

	return result, nil
} 