#!/bin/bash

# Đặt biến màu sắc để dễ đọc
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Cài đặt và chạy Auto Login Web ===${NC}"

# Đường dẫn đến thư mục autologin
AUTOLOGIN_DIR="$(pwd)"
WEB_DIR="$AUTOLOGIN_DIR/web"
FRONTEND_DIR="$WEB_DIR/frontend"

# Kiểm tra xem thư mục autologin có tồn tại không
if [ ! -d "$WEB_DIR" ]; then
    echo -e "${RED}Lỗi: Thư mục $WEB_DIR không tồn tại${NC}"
    exit 1
fi

# 1. Biên dịch batch_login
echo -e "${GREEN}1. Biên dịch batch_login...${NC}"
cd "$AUTOLOGIN_DIR"
go build -o batch_login ./cmd/batch_login/main.go

# 2. Tạo file processor.go đơn giản không phụ thuộc AccountProcessor
echo -e "${GREEN}2. Tạo file processor.go đơn giản...${NC}"
cd "$WEB_DIR"
cat > processor.go << 'EOF'
package main

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"
)

// SimulateProcessing là một hàm giả lập việc xử lý file Excel
// cho demo mà không cần gọi batch_login thực tế
func SimulateProcessing(
	excelPath string, 
	proxyPath string, 
	workers int, 
	processor interface{}, 
	onProgress func(progress float64, totalCount, successCount, failCount int),
) (string, string, error) {
	// Tạo tên file kết quả dựa trên timestamp
	timestamp := time.Now().Format("20060102_150405")
	successFilename := fmt.Sprintf("success_%s.xlsx", timestamp)
	failFilename := fmt.Sprintf("fail_%s.xlsx", timestamp)

	successPath := filepath.Join("results", successFilename)
	failPath := filepath.Join("results", failFilename)

	// Giả lập xử lý 10 tài khoản, mỗi tài khoản mất 1 giây
	totalAccounts := 10
	successCount := 0
	failCount := 0

	for i := 0; i < totalAccounts; i++ {
		time.Sleep(1 * time.Second)

		// Giả lập thành công hoặc thất bại ngẫu nhiên
		if i%3 == 0 {
			failCount++
		} else {
			successCount++
		}

		// Tính toán tiến trình
		progress := float64(i+1) / float64(totalAccounts)

		// Gọi callback để báo cáo tiến trình
		if onProgress != nil {
			onProgress(
				progress,
				totalAccounts,
				successCount,
				failCount,
			)
		}
	}

	return successPath, failPath, nil
}
EOF

# 3. Tạo thư mục public và file index.html nếu chưa tồn tại
echo -e "${GREEN}3. Chuẩn bị thư mục public...${NC}"
mkdir -p "$FRONTEND_DIR/public"
if [ ! -f "$FRONTEND_DIR/public/index.html" ]; then
    echo -e "${BLUE}Tạo file index.html...${NC}"
    cat > "$FRONTEND_DIR/public/index.html" << 'EOF'
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="theme-color" content="#000000" />
    <meta name="description" content="Auto Login Web App" />
    <title>Auto Login</title>
  </head>
  <body>
    <noscript>You need to enable JavaScript to run this app.</noscript>
    <div id="root"></div>
  </body>
</html>
EOF
fi

# 4. Cài đặt và build frontend
echo -e "${GREEN}4. Cài đặt và build frontend...${NC}"
cd "$FRONTEND_DIR"
if [ -d "node_modules" ]; then
    echo "Thư mục node_modules đã tồn tại, bỏ qua bước cài đặt"
else
    echo "Cài đặt dependencies frontend..."
    npm install --silent
fi

echo "Build frontend..."
npm run build

# 5. Chạy web server với GOPATH mode (đơn giản hơn)
echo -e "${GREEN}5. Khởi động web server...${NC}"
cd "$WEB_DIR"
echo -e "${BLUE}Server đang chạy tại http://localhost:8080${NC}"
echo -e "${BLUE}Nhấn Ctrl+C để dừng server${NC}"

# Tắt Go modules để chạy đơn giản hơn
GO111MODULE=off go run *.go

# Script sẽ kết thúc khi server bị dừng 