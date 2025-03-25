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

	// Phục vụ static assets từ React build
	r.Static("/static", "./frontend/build/static")

	// Phục vụ các file root như favicon, manifest
	r.StaticFile("/favicon.ico", "./frontend/build/favicon.ico")
	r.StaticFile("/manifest.json", "./frontend/build/manifest.json")
	r.StaticFile("/logo192.png", "./frontend/build/logo192.png")
	r.StaticFile("/logo512.png", "./frontend/build/logo512.png")

	// Xử lý tất cả các route còn lại bằng index.html
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
