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

# 2. Chuẩn bị module cho web
echo -e "${GREEN}2. Chuẩn bị Go module cho thư mục web...${NC}"
cd "$WEB_DIR"

# Xác định đúng module name từ go.mod
MODULE_NAME=$(grep "^module" go.mod | awk '{print $2}')
if [ -z "$MODULE_NAME" ]; then
    echo -e "${RED}Không thể xác định tên module từ go.mod${NC}"
    MODULE_NAME="github.com/khahanv2/smart-code-project/autologin/web"
    echo -e "${BLUE}Sử dụng tên module mặc định: $MODULE_NAME${NC}"
fi

# Tạo accountprocessor trong thư mục web
mkdir -p "$WEB_DIR/internal/accountprocessor"

# Kiểm tra xem thư mục accountprocessor trong thư mục gốc có tồn tại không
if [ -d "$AUTOLOGIN_DIR/internal/accountprocessor" ]; then
    echo -e "${BLUE}Sao chép accountprocessor từ thư mục gốc...${NC}"
    cp -f "$AUTOLOGIN_DIR/internal/accountprocessor/"*.go "$WEB_DIR/internal/accountprocessor/"
else
    echo -e "${RED}Không tìm thấy thư mục accountprocessor trong thư mục gốc!${NC}"
    # Tạo file accountprocessor.go cơ bản nếu không tìm thấy
    echo -e "${BLUE}Tạo file accountprocessor.go cơ bản...${NC}"
    cat > "$WEB_DIR/internal/accountprocessor/accountprocessor.go" << 'EOF'
package accountprocessor

import (
	"sync"
	"time"
)

// AccountProcessor đại diện cho bộ xử lý tài khoản với các trạng thái của chúng
type AccountProcessor struct {
	sync.Mutex
	TotalAccounts    int
	processingMap    map[string]bool
	successMap       map[string]bool
	failedMap        map[string]bool
	processingTimes  map[string]time.Time
}

// New tạo một AccountProcessor mới
func New() *AccountProcessor {
	return &AccountProcessor{
		processingMap:    make(map[string]bool),
		successMap:       make(map[string]bool),
		failedMap:        make(map[string]bool),
		processingTimes:  make(map[string]time.Time),
	}
}

// MarkProcessing đánh dấu một tài khoản đang được xử lý
func (ap *AccountProcessor) MarkProcessing(username string) {
	ap.Lock()
	defer ap.Unlock()
	ap.processingMap[username] = true
	ap.processingTimes[username] = time.Now()
}

// MarkSuccess đánh dấu một tài khoản đã xử lý thành công
func (ap *AccountProcessor) MarkSuccess(username string) {
	ap.Lock()
	defer ap.Unlock()
	delete(ap.processingMap, username)
	delete(ap.processingTimes, username)
	ap.successMap[username] = true
}

// MarkFailed đánh dấu một tài khoản đã xử lý thất bại
func (ap *AccountProcessor) MarkFailed(username string) {
	ap.Lock()
	defer ap.Unlock()
	delete(ap.processingMap, username)
	delete(ap.processingTimes, username)
	ap.failedMap[username] = true
}

// GetTotalAccounts trả về tổng số tài khoản
func (ap *AccountProcessor) GetTotalAccounts() int {
	return ap.TotalAccounts
}

// GetSuccessAccounts trả về số tài khoản thành công
func (ap *AccountProcessor) GetSuccessAccounts() int {
	ap.Lock()
	defer ap.Unlock()
	return len(ap.successMap)
}

// GetFailedAccounts trả về số tài khoản thất bại
func (ap *AccountProcessor) GetFailedAccounts() int {
	ap.Lock()
	defer ap.Unlock()
	return len(ap.failedMap)
}

// GetProcessingAccounts trả về số tài khoản đang xử lý
func (ap *AccountProcessor) GetProcessingAccounts() int {
	ap.Lock()
	defer ap.Unlock()
	return len(ap.processingMap)
}
EOF
fi

# 3. Sửa lỗi import trong processor.go
echo -e "${GREEN}3. Cập nhật import paths...${NC}"

# Sửa lỗi import trong processor.go
processor_file="$WEB_DIR/processor.go"
if [ -f "$processor_file" ]; then
    echo -e "${BLUE}Đang cập nhật import accountprocessor...${NC}"
    
    # Đọc nội dung hiện tại của file
    content=$(cat "$processor_file")
    
    # Tìm dòng import accountprocessor và thay thế bằng đường dẫn chính xác của module
    content=$(echo "$content" | sed "s|\"github.com/bongg/autologin/internal/accountprocessor\"|\"$MODULE_NAME/internal/accountprocessor\"|g")
    content=$(echo "$content" | sed "s|\"github.com/khahanv2/smart-code-project/autologin/internal/accountprocessor\"|\"$MODULE_NAME/internal/accountprocessor\"|g")
    content=$(echo "$content" | sed "s|\"github.com/khahanv2/smart-code-project/autologin/web/internal/accountprocessor\"|\"$MODULE_NAME/internal/accountprocessor\"|g")
    content=$(echo "$content" | sed "s|\"\./internal/accountprocessor\"|\"$MODULE_NAME/internal/accountprocessor\"|g")
    
    # Ghi lại nội dung đã sửa
    echo "$content" > "$processor_file"
    
    echo -e "${BLUE}Đã cập nhật import path thành '$MODULE_NAME/internal/accountprocessor'${NC}"
else
    echo -e "${RED}Không tìm thấy file processor.go${NC}"
    exit 1
fi

# Chạy go mod tidy để cập nhật dependencies
echo -e "${BLUE}Đang chạy go mod tidy...${NC}"
go mod tidy

# 4. Tạo thư mục public và file index.html nếu chưa tồn tại
echo -e "${GREEN}4. Chuẩn bị thư mục public...${NC}"
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

# 5. Cài đặt và build frontend
echo -e "${GREEN}5. Cài đặt và build frontend...${NC}"
cd "$FRONTEND_DIR"
if [ -d "node_modules" ]; then
    echo "Thư mục node_modules đã tồn tại, bỏ qua bước cài đặt"
else
    echo "Cài đặt dependencies frontend..."
    npm install --silent
fi

echo "Build frontend..."
npm run build

# 6. Chạy web server với mode cụ thể 
echo -e "${GREEN}6. Khởi động web server...${NC}"
cd "$WEB_DIR"
echo -e "${BLUE}Server đang chạy tại http://localhost:8080${NC}"
echo -e "${BLUE}Nhấn Ctrl+C để dừng server${NC}"

# Sử dụng -modfile=none để chạy mà không cần quy tắc strict của Go modules
GO111MODULE=on go run .

# Script sẽ kết thúc khi server bị dừng 