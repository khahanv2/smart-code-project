# Autologin Web Interface

Một giao diện web hiện đại cho ứng dụng Auto Login, kết hợp Material Design và Glassmorphism.

## Tính năng

- Upload file Excel chứa danh sách tài khoản
- Cấu hình số lượng workers và proxy
- Theo dõi tiến trình xử lý theo thời gian thực thông qua WebSocket
- Hiển thị biểu đồ và thống kê dễ hiểu
- Xem và tải xuống kết quả
- Theo dõi quá trình xử lý bằng log trực quan

## Công nghệ sử dụng

### Backend
- Go + Gin framework
- WebSocket cho cập nhật thời gian thực
- Quản lý nhiều job đồng thời

### Frontend
- React + Material UI
- Kết hợp phong cách Material Design và Glassmorphism
- Biểu đồ trực quan với Recharts
- Responsive design

## Cài đặt và Chạy

### Yêu cầu
- Go 1.16 trở lên
- Node.js 14.0 trở lên
- npm hoặc yarn

### Backend

```bash
# Cài đặt dependencies
cd web
go mod tidy

# Chạy server
go run main.go
```

### Frontend

```bash
# Cài đặt dependencies
cd web/frontend
npm install

# Chạy trong chế độ development
npm start

# Build cho production
npm run build
```

## Sử dụng

1. Truy cập http://localhost:8080 (mặc định)
2. Upload file Excel chứa danh sách tài khoản
3. Cấu hình các tham số (số lượng worker, proxy,...)
4. Bắt đầu xử lý và theo dõi tiến trình theo thời gian thực
5. Tải xuống kết quả khi hoàn tất

## Cấu trúc dự án

```
web/
├── main.go              # Web server với Gin
├── uploads/             # Thư mục lưu file đã upload
├── results/             # Thư mục lưu file kết quả
├── frontend/            # React frontend
│   ├── src/
│   │   ├── components/  # React components
│   │   ├── pages/       # Các trang của ứng dụng
│   │   └── App.js       # Component chính
│   ├── public/          # Static files
│   └── package.json     # Cấu hình và dependencies
└── go.mod               # Go module và dependencies
```