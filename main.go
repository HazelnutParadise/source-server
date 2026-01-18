package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 從 docker compose volume 取得 source 檔案路徑
var sourceDir = os.Getenv("SOURCE_DIR")
var supportStream = []string{"Chrome", "Firefox"}

func main() {
	// 如果未設定 SOURCE_DIR，使用預設 './sources'
	if sourceDir == "" {
		sourceDir = "./sources"
	}
	// 如果 sourceDir 不存在，則創建
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		os.MkdirAll(sourceDir, 0755)
	}
	fmt.Printf("Using SOURCE_DIR=%s\n", sourceDir)
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Origin"},
	}))
	r.GET("/*source", func(c *gin.Context) {
		source := c.Param("source")
		// c.Param for wildcard includes a leading '/'
		source = strings.TrimPrefix(source, "/")
		contentType := c.Query("content-type")
		// support POSIX-style paths in URL
		filePath := filepath.Clean(filepath.Join(sourceDir, filepath.FromSlash(source)))

		// 防止路徑穿越：確保檔案位於 sourceDir 之內
		absSourceDir, err := filepath.Abs(sourceDir)
		if err != nil {
			c.JSON(500, gin.H{"message": "server error"})
			return
		}
		absFilePath, err := filepath.Abs(filePath)
		if err != nil {
			c.JSON(500, gin.H{"message": "server error"})
			return
		}
		rel, err := filepath.Rel(absSourceDir, absFilePath)
		fmt.Printf("Request path=%q filePath=%q absSourceDir=%q absFilePath=%q rel=%q err=%v\n", source, filePath, absSourceDir, absFilePath, rel, err)
		if err != nil || strings.HasPrefix(rel, "..") {
			c.JSON(403, gin.H{"message": "forbidden"})
			return
		}

		file, err := os.Open(absFilePath)
		if err != nil {
			c.JSON(404, gin.H{"message": "file not found"})
			return
		}
		defer file.Close()

		// 如果請求的是目錄，回傳錯誤（或可改為提供目錄列舉）
		fileInfo, err := file.Stat()
		if err != nil {
			c.JSON(500, gin.H{"message": "could not get file info"})
			return
		}
		if fileInfo.IsDir() {
			// 回傳目錄內容簡短列舉
			entries, err := os.ReadDir(absFilePath)
			if err != nil {
				c.JSON(500, gin.H{"message": "could not read directory"})
				return
			}
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			c.JSON(400, gin.H{"message": "path is a directory", "entries": names})
			return
		}

		// 如果有 content-type 參數，則使用 content-type
		if contentType != "" {
			c.Header("Content-Type", contentType)
			c.File(file.Name())
			return
		}

		// 設置響應標頭
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", "attachment; filename="+source)
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// 使用串流傳輸（檢查 User-Agent 是否包含支援瀏覽器名稱）
		ua := c.Request.UserAgent()
		support := false
		for _, s := range supportStream {
			if strings.Contains(ua, s) {
				support = true
				break
			}
		}
		if support {
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, file)
				return err == nil
			})
		} else {
			// 如果前端不是支援的瀏覽器，則直接傳輸
			c.File(file.Name())
		}
	})
	r.Run(":8080")
}
