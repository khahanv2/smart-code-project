# Tài liệu CAPTCHA Solver

## Giới thiệu
CAPTCHA Solver là công cụ tự động tìm vị trí thanh trượt trong các CAPTCHA dạng trượt hình. Công cụ nhận dữ liệu từ file JSON có chứa hình ảnh base64 của hình trượt và nền, sau đó xử lý ảnh để xác định vị trí X cần đặt hình trượt.

## Nguyên lý hoạt động
Công cụ sử dụng kỹ thuật template matching (so khớp mẫu) để tìm vị trí chính xác nhất của hình trượt trên nền:
1. Giải mã chuỗi base64 thành ảnh
2. Tiền xử lý ảnh (grayscale, làm mờ, phát hiện cạnh)
3. Tìm kiếm vị trí khớp tốt nhất với nhiều tỷ lệ khác nhau
4. Trả về tọa độ X của điểm khớp

## Các tính năng chính

### 1. Socket Server (--service)
Chạy như một dịch vụ liên tục, cho phép nhiều client gửi yêu cầu xử lý CAPTCHA đồng thời:
```
./captcha_solver --service --port 9876
```
- Khởi tạo server lắng nghe kết nối trên cổng đã chỉ định
- Tự động tạo luồng riêng cho mỗi kết nối
- Nhận JSON từ client và trả về kết quả ngay lập tức
- Hỗ trợ nhiều kết nối đồng thời

### 2. Chế độ Pipe (--pipe)
Nhận dữ liệu từ stdin, hữu ích khi gọi từ ngôn ngữ khác như Golang:
```
cat example.json | ./captcha_solver --pipe
```
- Đọc chuỗi JSON từ stdin
- Xử lý và trả kết quả về stdout
- Thích hợp cho tích hợp với các ngôn ngữ khác

### 3. Xử lý hàng loạt (--batch)
Xử lý nhiều file JSON trong một thư mục:
```
./captcha_solver --batch thư_mục_chứa_json --output kết_quả.json
```
- Quét tất cả file JSON trong thư mục
- Xử lý đa luồng để tăng tốc độ
- Lưu kết quả vào file JSON nếu yêu cầu
- Hiển thị tiến trình và thời gian xử lý

### 4. Xử lý file đơn (--input) hoặc chuỗi JSON (--json)
Xử lý một file JSON hoặc chuỗi JSON trực tiếp:
```
./captcha_solver --input example.json
./captcha_solver --json '{"Data":{"Slider":"base64...","Background":"base64..."}}'
```
- Hỗ trợ đầu vào linh hoạt
- Trả về tọa độ X trực tiếp

### 5. Đa luồng để tăng hiệu suất
Tự động sử dụng đa luồng để xử lý nhanh hơn:
- Tận dụng nhiều CPU core
- Có thể tùy chỉnh số luồng với tham số --workers
- Phù hợp cho xử lý hàng loạt CAPTCHA

## Ví dụ sử dụng

### Xử lý CAPTCHA đơn lẻ
```
./captcha_solver --input example.json
```
Output: `126` (tọa độ X)

### Chạy như service và kết nối từ Python
```python
import socket
import json

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.connect(('localhost', 9876))
s.sendall((json_data + '\n').encode('utf-8'))
data = s.recv(1024)
result = json.loads(data.decode('utf-8'))
print(f"Tọa độ X: {result['x']}")
```

### Xử lý hàng loạt với 8 luồng
```
./captcha_solver --batch thư_mục_captcha --workers 8 --output kết_quả.json
```

## Lưu ý
- File thực thi đã được đóng gói sẵn mọi thư viện cần thiết
- Không cần cài đặt Python hoặc OpenCV
- Kích thước file lớn (~83MB) do chứa đầy đủ thư viện
- Hoạt động trên Linux không cần cài đặt thêm