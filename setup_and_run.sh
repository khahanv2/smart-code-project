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

# 2. Sửa file main.go để loại bỏ phụ thuộc vào accountprocessor
echo -e "${GREEN}2. Sửa file main.go...${NC}"
cd "$WEB_DIR"

# Tạo bản sao của main.go trước khi sửa
cp main.go main.go.bak

# Loại bỏ hoàn toàn dòng import accountprocessor
grep -v "accountprocessor" main.go > main.go.tmp
mv main.go.tmp main.go

# Đọc nội dung hiện tại của file
content=$(cat main.go)

# Thay đổi kiểu dữ liệu và các lời gọi
content=$(echo "$content" | sed 's/\*accountprocessor.AccountProcessor/interface{}/g')
content=$(echo "$content" | sed 's/accountprocessor.NewAccountProcessor()/nil/g')
content=$(echo "$content" | sed 's/job.Processor.GetTotalAccounts()/job.TotalAccounts/g')
content=$(echo "$content" | sed 's/job.Processor.GetSuccessAccounts()/job.SuccessCount/g')
content=$(echo "$content" | sed 's/job.Processor.GetFailedAccounts()/job.FailCount/g')

# Ghi lại nội dung đã sửa
echo "$content" > main.go.tmp
mv main.go.tmp main.go

# 3. Tạo file processor.go đơn giản
echo -e "${GREEN}3. Tạo file processor.go đơn giản...${NC}"
cat > processor.go << 'EOF'
package main

import (
	"fmt"
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

# 4. Cài đặt các gói cần thiết
echo -e "${GREEN}4. Cài đặt các gói Go cần thiết...${NC}"
cd "$WEB_DIR"
go get -u github.com/gin-gonic/gin
go get -u github.com/gin-contrib/cors
go get -u github.com/google/uuid
go get -u github.com/gorilla/websocket

# 5. Tạo thư mục public và file index.html nếu chưa tồn tại
echo -e "${GREEN}5. Chuẩn bị thư mục public...${NC}"
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

# 6. Cài đặt và build frontend
echo -e "${GREEN}6. Cài đặt và build frontend...${NC}"
cd "$FRONTEND_DIR"
if [ -d "node_modules" ]; then
    echo "Thư mục node_modules đã tồn tại, bỏ qua bước cài đặt"
else
    echo "Cài đặt dependencies frontend..."
    npm install --silent
fi

echo "Build frontend..."
npm run build

# 7. Tạo thư mục kết quả nếu chưa tồn tại
echo -e "${GREEN}7. Chuẩn bị thư mục kết quả...${NC}"
mkdir -p "$WEB_DIR/results"
mkdir -p "$WEB_DIR/uploads"

# 8. Chạy web server
echo -e "${GREEN}8. Khởi động web server...${NC}"
cd "$WEB_DIR"
echo -e "${BLUE}Server đang chạy tại http://localhost:8080${NC}"
echo -e "${BLUE}Nhấn Ctrl+C để dừng server${NC}"

# Chạy với Go module mode để sử dụng các gói đã cài đặt
GO111MODULE=on go run main.go processor.go

# Script sẽ kết thúc khi server bị dừng 