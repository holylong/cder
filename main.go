package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/atotto/clipboard"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/gen2brain/beeep"
)

const uploadFolder = "./documents"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)
var clientsMutex sync.Mutex

// calculateFileHash 计算文件的 SHA256 哈希值
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// sendNotification 发送桌面通知
func sendNotification(title, message string) {
	_ = beeep.Notify(title, message, "")
}

// uploadTextHandler 处理文本上传请求
func uploadTextHandler(c *gin.Context) {
	content := c.PostForm("content")
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	// 更新剪切板
	if err := clipboard.WriteAll(content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update clipboard"})
		return
	}

	// 成功通知
	sendNotification("Text Uploaded", fmt.Sprintf("Text saved: %s", content))

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"content": content,
	})
}

// uploadFileHandler 处理文件上传请求
func uploadFileHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}

	clientHash := c.PostForm("hash")
	if clientHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hash is required"})
		return
	}

	// 保存文件
	os.MkdirAll(uploadFolder, os.ModePerm)
	filePath := filepath.Join(uploadFolder, filepath.Base(file.Filename))
	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()

	// 模拟进度条
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer src.Close()

	buffer := make([]byte, 4096)
	totalBytes := int64(0)
	for {
		n, err := src.Read(buffer)
		if n > 0 {
			totalBytes += int64(n)
			_, _ = out.Write(buffer[:n])

			// 广播进度更新
			clientsMutex.Lock()
			for client := range clients {
				_ = client.WriteJSON(gin.H{
					"status":  "uploading",
					"progress": float64(totalBytes) / float64(file.Size) * 100.0,
				})
			}
			clientsMutex.Unlock()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
			return
		}
	}

	// 校验文件完整性
	serverHash, err := calculateFileHash(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate file hash"})
		return
	}

	if serverHash != clientHash {
		os.Remove(filePath) // 删除不完整文件
		c.JSON(http.StatusBadRequest, gin.H{"error": "File integrity check failed"})
		return
	}

	// 更新剪切板
	if err := clipboard.WriteAll(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update clipboard"})
		return
	}

	// 成功通知
	sendNotification("File Uploaded", fmt.Sprintf("File saved: %s", filePath))

	// 广播完成消息
	clientsMutex.Lock()
	for client := range clients {
		_ = client.WriteJSON(gin.H{
			"status": "completed",
			"path":   filePath,
		})
	}
	clientsMutex.Unlock()

	// 成功返回
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"path":   filePath,
	})
}

// websocketHandler 处理 WebSocket 连接
func websocketHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("Failed to upgrade connection:", err)
		return
	}
	defer conn.Close()

	clientsMutex.Lock()
	clients[conn] = true
	clientsMutex.Unlock()

	// 监听客户端消息（保持连接）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	clientsMutex.Lock()
	delete(clients, conn)
	clientsMutex.Unlock()
}

func main() {
	// 初始化 Gin
	r := gin.Default()

	// 文件上传接口
	r.POST("/upload/file", uploadFileHandler)

	// 文本上传接口
	r.POST("/upload/text", uploadTextHandler)

	// WebSocket 接口
	r.GET("/ws", websocketHandler)

	// 启动服务
	port := "5000"
	fmt.Printf("Server running on http://0.0.0.0:%s\n", port)
	r.Run(":" + port)
}