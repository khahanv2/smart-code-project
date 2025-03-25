package accountprocessor

import (
	"fmt"
	"sync"
	"time"

	"github.com/bongg/autologin/logger"
)

// AccountProcessor là struct quản lý việc đếm tài khoản một cách chính xác
type AccountProcessor struct {
	// Biến đếm cơ bản
	TotalAccounts   int
	SuccessAccounts int
	FailedAccounts  int

	// Maps theo dõi trạng thái
	AllAccounts        map[string]bool      // Tất cả tài khoản hợp lệ
	InProgressAccounts map[string]time.Time // Đang xử lý
	SuccessMap         map[string]bool      // Đã thành công
	FailedMap          map[string]bool      // Đã thất bại

	// Mutex để bảo vệ dữ liệu
	mu sync.Mutex
}

// NewAccountProcessor tạo một AccountProcessor mới
func NewAccountProcessor() *AccountProcessor {
	return &AccountProcessor{
		TotalAccounts:      0,
		SuccessAccounts:    0,
		FailedAccounts:     0,
		AllAccounts:        make(map[string]bool),
		InProgressAccounts: make(map[string]time.Time),
		SuccessMap:         make(map[string]bool),
		FailedMap:          make(map[string]bool),
	}
}

// InitializeFromExcel thiết lập danh sách tài khoản từ dữ liệu Excel
func (p *AccountProcessor) InitializeFromExcel(rows [][]string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	validAccounts := 0
	for _, row := range rows {
		// Đảm bảo có đủ cột
		if len(row) < 3 {
			continue
		}

		username := row[1] // Cột 2 (index 1) là tài khoản
		password := row[2] // Cột 3 (index 2) là mật khẩu

		// Kiểm tra tài khoản hoặc mật khẩu trống
		if username == "" || password == "" {
			continue
		}

		// Đánh dấu là tài khoản hợp lệ
		p.AllAccounts[username] = true
		validAccounts++
	}
	p.TotalAccounts = validAccounts
	logger.Log.Info().Int("validAccounts", validAccounts).Msg("Đã khởi tạo processor với số tài khoản hợp lệ")
}

// MarkProcessing đánh dấu tài khoản đang được xử lý
func (p *AccountProcessor) MarkProcessing(username string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.AllAccounts[username]; exists {
		p.InProgressAccounts[username] = time.Now()
	} else {
		// Nếu tài khoản không nằm trong danh sách hợp lệ, vẫn thêm vào
		p.AllAccounts[username] = true
		p.InProgressAccounts[username] = time.Now()
		p.TotalAccounts++
		logger.Log.Warn().Str("username", username).Msg("Đánh dấu tài khoản không nằm trong danh sách ban đầu")
	}
}

// MarkSuccess đánh dấu tài khoản đã xử lý thành công
func (p *AccountProcessor) MarkSuccess(username string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Xóa khỏi danh sách đang xử lý
	delete(p.InProgressAccounts, username)

	// Đảm bảo tài khoản không bị đếm trùng
	if _, exists := p.SuccessMap[username]; !exists && !p.FailedMap[username] {
		p.SuccessMap[username] = true
		p.SuccessAccounts++
	} else if p.FailedMap[username] {
		// Nếu đã đánh dấu là thất bại, chuyển sang thành công
		delete(p.FailedMap, username)
		p.FailedAccounts--
		p.SuccessMap[username] = true
		p.SuccessAccounts++
		logger.Log.Warn().Str("username", username).Msg("Tài khoản chuyển từ thất bại sang thành công")
	} else {
		logger.Log.Warn().Str("username", username).Msg("Tài khoản đã được đánh dấu thành công trước đó")
	}
}

// MarkFailed đánh dấu tài khoản đã xử lý thất bại
func (p *AccountProcessor) MarkFailed(username string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Xóa khỏi danh sách đang xử lý
	delete(p.InProgressAccounts, username)

	// Đảm bảo tài khoản không bị đếm trùng
	if _, exists := p.FailedMap[username]; !exists && !p.SuccessMap[username] {
		p.FailedMap[username] = true
		p.FailedAccounts++
	} else if p.SuccessMap[username] {
		// Nếu đã đánh dấu là thành công, giữ nguyên thành công
		logger.Log.Warn().Str("username", username).Msg("Tài khoản đã thành công nhưng sau đó được đánh dấu thất bại - giữ thành công")
	} else {
		logger.Log.Warn().Str("username", username).Msg("Tài khoản đã được đánh dấu thất bại trước đó")
	}
}

// GetTotalAccounts trả về tổng số tài khoản
func (p *AccountProcessor) GetTotalAccounts() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.TotalAccounts
}

// GetSuccessAccounts trả về số tài khoản đăng nhập thành công
func (p *AccountProcessor) GetSuccessAccounts() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.SuccessAccounts
}

// GetFailedAccounts trả về số tài khoản đăng nhập thất bại
func (p *AccountProcessor) GetFailedAccounts() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.FailedAccounts
}

// Reconcile kiểm tra tính nhất quán và tự động điều chỉnh nếu cần
func (p *AccountProcessor) Reconcile() (bool, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	issues := []string{}

	// Kiểm tra tài khoản bị treo trong xử lý quá lâu (>2 phút)
	now := time.Now()
	for username, startTime := range p.InProgressAccounts {
		if now.Sub(startTime) > 2*time.Minute {
			issues = append(issues, fmt.Sprintf("Tài khoản %s bị treo trong xử lý", username))
			// Tự điều chỉnh: đánh dấu là thất bại
			delete(p.InProgressAccounts, username)
			if !p.SuccessMap[username] && !p.FailedMap[username] {
				p.FailedMap[username] = true
				p.FailedAccounts++
			}
		}
	}

	// Kiểm tra tài khoản bị đếm trùng
	for username := range p.SuccessMap {
		if p.FailedMap[username] {
			issues = append(issues, fmt.Sprintf("Tài khoản %s bị đếm cả thành công và thất bại", username))
			// Tự điều chỉnh: giữ thành công, xóa thất bại
			delete(p.FailedMap, username)
			p.FailedAccounts--
		}
	}

	// Kiểm tra tài khoản chưa được xử lý
	for username := range p.AllAccounts {
		if !p.SuccessMap[username] && !p.FailedMap[username] && p.InProgressAccounts[username] == (time.Time{}) {
			issues = append(issues, fmt.Sprintf("Tài khoản %s chưa được xử lý", username))
			// Tự điều chỉnh: đánh dấu là thất bại
			p.FailedMap[username] = true
			p.FailedAccounts++
		}
	}

	// Kiểm tra tổng số
	if p.SuccessAccounts+p.FailedAccounts != p.TotalAccounts {
		issues = append(issues, fmt.Sprintf("Tổng số không khớp: %d thành công + %d thất bại != %d tổng",
			p.SuccessAccounts, p.FailedAccounts, p.TotalAccounts))

		// Tự điều chỉnh giá trị đếm
		actualSuccess := len(p.SuccessMap)
		actualFailed := len(p.FailedMap)

		// Nếu số lượng trong map không khớp với biến đếm
		if p.SuccessAccounts != actualSuccess {
			issues = append(issues, fmt.Sprintf("Số tài khoản thành công không khớp: biến đếm = %d, map = %d",
				p.SuccessAccounts, actualSuccess))
			p.SuccessAccounts = actualSuccess
		}

		if p.FailedAccounts != actualFailed {
			issues = append(issues, fmt.Sprintf("Số tài khoản thất bại không khớp: biến đếm = %d, map = %d",
				p.FailedAccounts, actualFailed))
			p.FailedAccounts = actualFailed
		}

		// Kiểm tra lại tổng sau khi điều chỉnh
		if (p.SuccessAccounts + p.FailedAccounts) != p.TotalAccounts {
			issues = append(issues, fmt.Sprintf("Sau khi điều chỉnh, tổng vẫn không khớp: %d thành công + %d thất bại != %d tổng",
				p.SuccessAccounts, p.FailedAccounts, p.TotalAccounts))

			// Đếm lại tổng từ map
			totalFromMaps := len(p.AllAccounts)
			if totalFromMaps != p.TotalAccounts {
				issues = append(issues, fmt.Sprintf("Tổng số tài khoản không khớp: biến đếm = %d, số tài khoản thực tế = %d",
					p.TotalAccounts, totalFromMaps))
				p.TotalAccounts = totalFromMaps
			}
		}
	}

	return len(issues) == 0, issues
}

// PrintStatistics in ra thống kê hiện tại
func (p *AccountProcessor) PrintStatistics() {
	p.mu.Lock()
	defer p.mu.Unlock()

	logger.Log.Info().
		Int("total", p.TotalAccounts).
		Int("success", p.SuccessAccounts).
		Int("failed", p.FailedAccounts).
		Int("inProgress", len(p.InProgressAccounts)).
		Msg("Thống kê tài khoản hiện tại")
}
