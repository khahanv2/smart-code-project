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

# 2. Tạo lại file main.go từ đầu
echo -e "${GREEN}2. Tạo lại file main.go...${NC}"
cd "$WEB_DIR"

# Sao lưu file cũ
cp main.go main.go.original.bak

# Tạo file mới hoàn toàn
cat > main.go << 'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Job represents a processing job
type Job struct {
	ID              string      `json:"id"`
	Status          string      `json:"status"` // "pending", "processing", "completed", "failed"
	UploadedFile    string      `json:"uploadedFile"`
	ProxyFile       string      `json:"proxyFile"`
	Workers         int         `json:"workers"`
	StartTime       time.Time   `json:"startTime"`
	EndTime         time.Time   `json:"endTime"`
	SuccessFile     string      `json:"successFile"`
	FailFile        string      `json:"failFile"`
	Progress        float64     `json:"progress"`
	Processor       interface{} `json:"-"`
	TotalAccounts   int         `json:"totalAccounts"`
	SuccessCount    int         `json:"successCount"`
	FailCount       int         `json:"failCount"`
	ProcessingCount int         `json:"processingCount"`
}

var (
	jobs        = make(map[string]*Job)
	jobsMutex   sync.RWMutex
	clients     = make(map[string][]*websocket.Conn)
	clientMutex sync.RWMutex
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func main() {
	// Tạo các thư mục cần thiết
	os.MkdirAll("uploads", os.ModePerm)
	os.MkdirAll("results", os.ModePerm)

	r := gin.Default()

	// Cấu hình CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Endpoint API
	api := r.Group("/api")
	{
		api.GET("/jobs", getJobs)
		api.POST("/jobs", createJob)
		api.GET("/jobs/:id", getJob)
		api.GET("/jobs/:id/download/:type", downloadResult)
		api.GET("/ws/jobs/:id", wsJobStatus)
	}

	// Phục vụ file tĩnh
	r.Static("/uploads", "./uploads")
	r.Static("/results", "./results")
	r.Static("/", "./frontend/build")

	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/build/index.html")
	})

	fmt.Println("Server started on :8080")
	r.Run(":8080")
}

func getJobs(c *gin.Context) {
	jobsMutex.RLock()
	defer jobsMutex.RUnlock()

	jobsList := make([]*Job, 0, len(jobs))
	for _, job := range jobs {
		jobsList = append(jobsList, job)
	}

	c.JSON(http.StatusOK, jobsList)
}

func getJob(c *gin.Context) {
	id := c.Param("id")

	jobsMutex.RLock()
	job, exists := jobs[id]
	jobsMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func createJob(c *gin.Context) {
	// Parse form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	// Get workers count
	workersStr := c.Request.FormValue("workers")
	workers, err := strconv.Atoi(workersStr)
	if err != nil || workers <= 0 {
		workers = 10 // Default value
	}

	// Excel file
	excelFile, header, err := c.Request.FormFile("excelFile")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Excel file is required"})
		return
	}
	defer excelFile.Close()

	// Create uploads directory if not exists
	if err := os.MkdirAll("uploads", os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create uploads directory"})
		return
	}

	// Save Excel file
	excelFilename := filepath.Join("uploads", header.Filename)
	out, err := os.Create(excelFilename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, excelFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Check for proxy file (optional)
	var proxyFilename string
	if proxyFile, header, err := c.Request.FormFile("proxyFile"); err == nil {
		defer proxyFile.Close()
		proxyFilename = filepath.Join("uploads", header.Filename)
		out, err := os.Create(proxyFilename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save proxy file"})
			return
		}
		defer out.Close()

		_, err = io.Copy(out, proxyFile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save proxy file"})
			return
		}
	}

	// Create new job
	job := &Job{
		ID:           uuid.New().String(),
		Status:       "pending",
		UploadedFile: excelFilename,
		ProxyFile:    proxyFilename,
		Workers:      workers,
		StartTime:    time.Now(),
		Progress:     0,
		Processor:    nil,
	}

	// Store job
	jobsMutex.Lock()
	jobs[job.ID] = job
	jobsMutex.Unlock()

	// Start processing in background
	go processJob(job)

	// Return job ID
	c.JSON(http.StatusOK, gin.H{"id": job.ID})
}

func downloadResult(c *gin.Context) {
	id := c.Param("id")
	fileType := c.Param("type")

	jobsMutex.RLock()
	job, exists := jobs[id]
	jobsMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	var filePath string
	if fileType == "success" {
		filePath = job.SuccessFile
	} else if fileType == "fail" {
		filePath = job.FailFile
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type"})
		return
	}

	if filePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filePath)
}

func wsJobStatus(c *gin.Context) {
	id := c.Param("id")

	jobsMutex.RLock()
	_, exists := jobs[id]
	jobsMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to set websocket upgrade: %v", err)
		return
	}

	// Register client
	clientMutex.Lock()
	if clients[id] == nil {
		clients[id] = make([]*websocket.Conn, 0)
	}
	clients[id] = append(clients[id], conn)
	clientMutex.Unlock()

	// Remove client on disconnect
	conn.SetCloseHandler(func(code int, text string) error {
		clientMutex.Lock()
		for i, client := range clients[id] {
			if client == conn {
				clients[id] = append(clients[id][:i], clients[id][i+1:]...)
				break
			}
		}
		clientMutex.Unlock()
		return nil
	})

	// Keep connection open
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func processJob(job *Job) {
	// Mark job as processing
	jobsMutex.Lock()
	job.Status = "processing"
	jobsMutex.Unlock()

	// Broadcast job update
	broadcastJobUpdate(job)

	// Setup progress callback
	onProgress := func(progress float64, totalCount, successCount, failCount int) {
		jobsMutex.Lock()
		job.Progress = progress
		job.TotalAccounts = totalCount
		job.SuccessCount = successCount
		job.FailCount = failCount
		jobsMutex.Unlock()

		broadcastJobUpdate(job)
	}

	// Process
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
			jobsMutex.Lock()
			job.Status = "failed"
			job.EndTime = time.Now()
			jobsMutex.Unlock()
			broadcastJobUpdate(job)
		}
	}()

	// Sử dụng SimulateProcessing thay vì ProcessExcelFile cho demo
	successFile, failFile, err := SimulateProcessing(
		job.UploadedFile,
		job.ProxyFile,
		job.Workers,
		job.Processor,
		onProgress,
	)

	jobsMutex.Lock()
	defer jobsMutex.Unlock()

	if err != nil {
		job.Status = "failed"
		log.Printf("Error processing job: %v", err)
	} else {
		job.Status = "completed"
		job.SuccessFile = successFile
		job.FailFile = failFile
	}

	job.EndTime = time.Now()
	broadcastJobUpdate(job)
}

func broadcastJobUpdate(job *Job) {
	// Create a copy for broadcasting
	jobData, _ := json.Marshal(job)

	clientMutex.RLock()
	defer clientMutex.RUnlock()

	for _, conn := range clients[job.ID] {
		conn.WriteMessage(websocket.TextMessage, jobData)
	}
}
EOF

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
go run main.go processor.go

# Script sẽ kết thúc khi server bị dừng 