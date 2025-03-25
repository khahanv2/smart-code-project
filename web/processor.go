package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/bongg/autologin/internal/accountprocessor"
)

// ProcessExcelFile sử dụng batch_login để xử lý file Excel
// và trả về các file kết quả
func ProcessExcelFile(excelPath string, proxyPath string, workers int, processor *accountprocessor.AccountProcessor) (string, string, error) {
	// Tạo tên file kết quả dựa trên timestamp
	timestamp := time.Now().Format("20060102_150405")
	successFilename := fmt.Sprintf("success_%s.xlsx", timestamp)
	failFilename := fmt.Sprintf("fail_%s.xlsx", timestamp)

	successPath := filepath.Join("results", successFilename)
	failPath := filepath.Join("results", failFilename)

	// Chuẩn bị lệnh batch_login
	args := []string{excelPath, fmt.Sprintf("%d", workers)}

	// Thực thi batch_login như một subprocess
	cmd := exec.Command("../batch_login", args...)

	// Lưu kết quả đầu ra
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("error running batch_login: %v\nOutput: %s", err, string(output))
	}

	return successPath, failPath, nil
}

// SimulateProcessing là một hàm giả lập việc xử lý file Excel
// cho demo mà không cần gọi batch_login thực tế
func SimulateProcessing(excelPath string, proxyPath string, workers int, processor *accountprocessor.AccountProcessor, onProgress func(progress float64, totalCount, successCount, failCount int)) (string, string, error) {
	// Tạo tên file kết quả dựa trên timestamp
	timestamp := time.Now().Format("20060102_150405")
	successFilename := fmt.Sprintf("success_%s.xlsx", timestamp)
	failFilename := fmt.Sprintf("fail_%s.xlsx", timestamp)

	successPath := filepath.Join("results", successFilename)
	failPath := filepath.Join("results", failFilename)

	// Giả lập xử lý 10 tài khoản, mỗi tài khoản mất 1 giây
	totalAccounts := 10
	processor.TotalAccounts = totalAccounts

	for i := 0; i < totalAccounts; i++ {
		time.Sleep(1 * time.Second)

		// Giả lập thành công hoặc thất bại ngẫu nhiên
		username := fmt.Sprintf("user_%d", i)
		processor.MarkProcessing(username)

		// Giả lập một số tài khoản thành công, một số thất bại
		if i%3 == 0 {
			processor.MarkFailed(username)
		} else {
			processor.MarkSuccess(username)
		}

		// Tính toán tiến trình
		progress := float64(i+1) / float64(totalAccounts)

		// Gọi callback để báo cáo tiến trình
		if onProgress != nil {
			onProgress(
				progress,
				processor.GetTotalAccounts(),
				processor.GetSuccessAccounts(),
				processor.GetFailedAccounts(),
			)
		}
	}

	return successPath, failPath, nil
}
