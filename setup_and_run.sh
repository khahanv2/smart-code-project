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

# 2. Sửa lỗi module và import paths
echo -e "${GREEN}2. Cập nhật module paths...${NC}"
# Chuyển đến thư mục web
cd "$WEB_DIR"
echo -e "${BLUE}Đang cập nhật go.mod...${NC}"

# Cập nhật require trong go.mod để trỏ đến đúng module path
grep -q "replace github.com/khahanv2/smart-code-project" go.mod
if [ $? -ne 0 ]; then
    echo -e "replace github.com/khahanv2/smart-code-project => ../.." >> go.mod
    echo -e "${BLUE}Đã thêm replace directive vào go.mod${NC}"
fi

# Chạy go mod tidy để cập nhật dependencies
echo -e "${BLUE}Đang chạy go mod tidy...${NC}"
go mod tidy

# Sửa lỗi import trong processor.go
processor_file="$WEB_DIR/processor.go"
if [ -f "$processor_file" ]; then
    echo -e "${BLUE}Đang kiểm tra import accountprocessor...${NC}"
    
    # Sửa import path
    sed -i 's|github.com/bongg/autologin/internal/accountprocessor|github.com/khahanv2/smart-code-project/autologin/internal/accountprocessor|g' "$processor_file"
    sed -i 's|github.com/khahanv2/smart-code-project/autologin/internal/accountprocessor|github.com/khahanv2/smart-code-project/autologin/internal/accountprocessor|g' "$processor_file"
    
    echo -e "${BLUE}Đã cập nhật import paths${NC}"
else
    echo -e "${RED}Không tìm thấy file processor.go${NC}"
    exit 1
fi

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

# 5. Chạy web server
echo -e "${GREEN}5. Khởi động web server...${NC}"
cd "$WEB_DIR"
echo -e "${BLUE}Server đang chạy tại http://localhost:8080${NC}"
echo -e "${BLUE}Nhấn Ctrl+C để dừng server${NC}"
go run *.go

# Script sẽ kết thúc khi server bị dừng 