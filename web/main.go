package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bongg/autologin/internal/accountprocessor"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Job represents a processing job
type Job struct {
	ID              string                             `json:"id"`
	Status          string                             `json:"status"` // "pending", "processing", "completed", "failed"
	UploadedFile    string                             `json:"uploadedFile"`
	ProxyFile       string                             `json:"proxyFile"`
	Workers         int                                `json:"workers"`
	StartTime       time.Time                          `json:"startTime"`
	EndTime         time.Time                          `json:"endTime"`
	SuccessFile     string                             `json:"successFile"`
	FailFile        string                             `json:"failFile"`
	Progress        float64                            `json:"progress"`
	Processor       *accountprocessor.AccountProcessor `json:"-"`
	TotalAccounts   int                                `json:"totalAccounts"`
	SuccessCount    int                                `json:"successCount"`
	FailCount       int                                `json:"failCount"`
	ProcessingCount int                                `json:"processingCount"`
}

// JobManager manages all jobs
type JobManager struct {
	jobs       map[string]*Job
	clients    map[string][]*websocket.Conn
	mutex      sync.Mutex
	uploadsDir string
	resultsDir string
}

// NewJobManager creates a new job manager
func NewJobManager() *JobManager {
	// Ensure directories exist
	uploadsDir := "uploads"
	resultsDir := "results"

	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		os.Mkdir(uploadsDir, 0755)
	}

	if _, err := os.Stat(resultsDir); os.IsNotExist(err) {
		os.Mkdir(resultsDir, 0755)
	}

	return &JobManager{
		jobs:       make(map[string]*Job),
		clients:    make(map[string][]*websocket.Conn),
		uploadsDir: uploadsDir,
		resultsDir: resultsDir,
	}
}

// AddJob adds a new job
func (jm *JobManager) AddJob(job *Job) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	jm.jobs[job.ID] = job
}

// GetJob returns a job by ID
func (jm *JobManager) GetJob(id string) (*Job, bool) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	job, exists := jm.jobs[id]
	return job, exists
}

// UpdateJob updates a job
func (jm *JobManager) UpdateJob(job *Job) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	jm.jobs[job.ID] = job
	jm.notifyClients(job.ID, job)
}

// AddClient adds a WebSocket client for a job
func (jm *JobManager) AddClient(jobID string, conn *websocket.Conn) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	jm.clients[jobID] = append(jm.clients[jobID], conn)
}

// RemoveClient removes a WebSocket client
func (jm *JobManager) RemoveClient(jobID string, conn *websocket.Conn) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	clients := jm.clients[jobID]
	for i, c := range clients {
		if c == conn {
			jm.clients[jobID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
}

// notifyClients sends an update to all clients for a job
func (jm *JobManager) notifyClients(jobID string, data interface{}) {
	clients := jm.clients[jobID]
	for _, conn := range clients {
		err := conn.WriteJSON(data)
		if err != nil {
			log.Printf("Error sending to WebSocket: %v", err)
			conn.Close()
			jm.RemoveClient(jobID, conn)
		}
	}
}

// Global JobManager
var jobManager *JobManager

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Handler for uploading Excel file
func handleUpload(c *gin.Context) {
	// Get form values
	workersStr := c.PostForm("workers")
	workers := 1
	if workersStr != "" {
		fmt.Sscanf(workersStr, "%d", &workers)
	}
	if workers < 1 {
		workers = 1
	}

	// Handle Excel file upload
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Generate unique ID for the job
	jobID := uuid.New().String()

	// Save the uploaded file
	uploadPath := filepath.Join(jobManager.uploadsDir, jobID+"_"+header.Filename)
	out, err := os.Create(uploadPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Check for proxy file
	var proxyPath string
	proxyFile, proxyHeader, err := c.Request.FormFile("proxy")
	if err == nil && proxyFile != nil {
		defer proxyFile.Close()
		proxyPath = filepath.Join(jobManager.uploadsDir, jobID+"_"+proxyHeader.Filename)
		proxyOut, err := os.Create(proxyPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save proxy file"})
			return
		}
		defer proxyOut.Close()
		_, err = io.Copy(proxyOut, proxyFile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save proxy file"})
			return
		}
	}

	// Create a new job
	job := &Job{
		ID:           jobID,
		Status:       "pending",
		UploadedFile: uploadPath,
		ProxyFile:    proxyPath,
		Workers:      workers,
		StartTime:    time.Now(),
		Processor:    accountprocessor.NewAccountProcessor(),
	}

	// Add job to manager
	jobManager.AddJob(job)

	// Start processing in background
	go processJob(job)

	c.JSON(http.StatusOK, gin.H{
		"jobId":  jobID,
		"status": "pending",
	})
}

// Handler for WebSocket connections
func handleWebSocket(c *gin.Context) {
	jobID := c.Param("id")

	job, exists := jobManager.GetJob(jobID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}

	// Add client to job
	jobManager.AddClient(jobID, conn)

	// Send initial job data
	conn.WriteJSON(job)

	// Listen for close
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				jobManager.RemoveClient(jobID, conn)
				break
			}
		}
	}()
}

// Handler for job status
func getJobStatus(c *gin.Context) {
	jobID := c.Param("id")

	job, exists := jobManager.GetJob(jobID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// Handler for downloading result files
func handleDownload(c *gin.Context) {
	jobID := c.Param("id")
	fileType := c.Param("type") // "success" or "fail"

	job, exists := jobManager.GetJob(jobID)
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

// Handler for listing all jobs
func listJobs(c *gin.Context) {
	jobManager.mutex.Lock()
	defer jobManager.mutex.Unlock()

	jobs := make([]Job, 0, len(jobManager.jobs))
	for _, job := range jobManager.jobs {
		jobs = append(jobs, *job)
	}

	c.JSON(http.StatusOK, jobs)
}

// Background processor for jobs
func processJob(job *Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in job %s: %v", job.ID, r)
			job.Status = "failed"
			jobManager.UpdateJob(job)
		}
	}()

	// Update job status
	job.Status = "processing"
	jobManager.UpdateJob(job)

	// Callback function to update progress
	onProgress := func(progress float64, totalCount, successCount, failCount int) {
		job.Progress = progress
		job.TotalAccounts = totalCount
		job.SuccessCount = successCount
		job.FailCount = failCount
		job.ProcessingCount = totalCount - successCount - failCount
		jobManager.UpdateJob(job)
	}

	// Sử dụng SimulateProcessing thay vì ProcessExcelFile cho demo
	// Trong production, hãy dùng ProcessExcelFile
	successFile, failFile, err := SimulateProcessing(
		job.UploadedFile,
		job.ProxyFile,
		job.Workers,
		job.Processor,
		onProgress,
	)

	if err != nil {
		log.Printf("Error processing job %s: %v", job.ID, err)
		job.Status = "failed"
		jobManager.UpdateJob(job)
		return
	}

	job.Status = "completed"
	job.EndTime = time.Now()
	job.SuccessFile = successFile
	job.FailFile = failFile
	job.Progress = 1.0

	// Cập nhật số liệu từ processor
	job.TotalAccounts = job.Processor.GetTotalAccounts()
	job.SuccessCount = job.Processor.GetSuccessAccounts()
	job.FailCount = job.Processor.GetFailedAccounts()
	job.ProcessingCount = 0

	// Update final job status
	jobManager.UpdateJob(job)
}

// Custom WebSocket logger that integrates with the job system
type WebSocketLogger struct {
	JobID string
}

func (l *WebSocketLogger) Write(p []byte) (n int, err error) {
	// Try to parse the log message
	var logData map[string]interface{}
	if err := json.Unmarshal(p, &logData); err == nil {
		// Send to WebSocket clients
		jobManager.notifyClients(l.JobID, gin.H{
			"type": "log",
			"data": logData,
		})
	}

	// Also write to standard output
	return os.Stdout.Write(p)
}

func main() {
	// Initialize the job manager
	jobManager = NewJobManager()

	// Create Gin router
	router := gin.Default()

	// CORS setup
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// API routes
	api := router.Group("/api")
	{
		api.POST("/upload", handleUpload)
		api.GET("/jobs", listJobs)
		api.GET("/job/:id", getJobStatus)
		api.GET("/download/:type/:id", handleDownload)
	}

	// WebSocket route
	router.GET("/ws/:id", handleWebSocket)

	// Serve static files from the frontend folder
	router.StaticFS("/app", http.Dir("./frontend/build"))

	// Fallback to the SPA index file
	router.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.File("./frontend/build/index.html")
		}
	})

	// Start the server
	log.Println("Starting server on http://localhost:8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
