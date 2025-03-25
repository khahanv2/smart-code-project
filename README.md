# Autologin Project

Dự án này cung cấp một giải pháp tự động đăng nhập và xử lý captcha được phát triển bằng Go.

## Tính năng chính

- Tự động lấy thông tin từ trang đăng nhập (token, cookies)
- Xử lý slider captcha
- Hỗ trợ sử dụng proxy
- Đăng nhập hàng loạt từ danh sách tài khoản trong file Excel
- Công cụ trích xuất captcha

## Cấu trúc dự án

```
.
├── captcha/           # Xử lý captcha
├── client/            # HTTP client và logic xử lý đăng nhập
├── cmd/               # Các lệnh khác nhau
│   ├── auto_login/    # Tự động đăng nhập
│   ├── batch_login/   # Đăng nhập hàng loạt từ file Excel
│   └── extract_captcha/ # Trích xuất captcha
├── config/            # Quản lý cấu hình
├── results/           # Kết quả đăng nhập (thành công/thất bại)
├── utils/             # Tiện ích
├── batch_login        # Binary file đăng nhập hàng loạt
├── captcha_solver     # Binary file giải captcha
├── go.mod             # Quản lý dependencies Go
├── go.sum             # Checksums cho dependencies
├── main.go            # File chương trình chính
├── proxy.txt          # Danh sách proxy
└── README.md          # Tài liệu hướng dẫn
```

## Cài đặt

```bash
# Clone repository
git clone https://github.com/khahanv2/smart-code-project.git
cd smart-code-project

# Build
go build -o autologin main.go
```

## Sử dụng cơ bản

### Chạy chương trình chính

```bash
./autologin
```

### Đăng nhập hàng loạt từ file Excel

```bash
./batch_login -f Book1.xlsx
```

## Cấu hình

Tệp cấu hình có thể được truyền vào khi khởi tạo client. Nếu không cung cấp, chương trình sẽ sử dụng giá trị mặc định.

## Đóng góp

Đóng góp và báo cáo lỗi luôn được chào đón. Vui lòng mở Issue hoặc gửi Pull Request để cải thiện dự án này.