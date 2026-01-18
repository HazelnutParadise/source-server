package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
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

		// 決定 MIME 類型（以 query content-type 優先，否則由副檔名或前 512 bytes 偵測）
		mimeType := contentType
		if mimeType == "" {
			ext := strings.ToLower(filepath.Ext(source))
			if ext != "" {
				mimeType = mime.TypeByExtension(ext)
			}
			if mimeType == "" {
				// sniff
				buf := make([]byte, 512)
				n, _ := file.ReadAt(buf, 0)
				mimeType = http.DetectContentType(buf[:n])
				file.Seek(0, io.SeekStart)
			}
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		c.Header("Content-Type", mimeType)

		// 根據 MIME 決定是否為 media（audio/video）
		isMedia := strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/")
		// 如果是 media，回傳內嵌；非 media 預設為 attachment（可以用 ?download=1 強制下載）
		if isMedia && c.Query("download") != "1" {
			c.Header("Content-Disposition", "inline; filename="+source)
		} else {
			c.Header("Content-Disposition", "attachment; filename="+source)
		}
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")

		// 自動切換：若有 Range header（表示 client 想要部分內容 / 續傳）或是 media（讓瀏覽器能尋址播放）就使用 ServeContent
		chunked := c.Query("chunked") == "1"
		if c.Request.Header.Get("Range") != "" || isMedia || !chunked {
			http.ServeContent(c.Writer, c.Request, source, fileInfo.ModTime(), file)
			return
		}

		// 否則若使用 chunked 模式，採分塊串流（不設定 Content-Length）
		buf := make([]byte, 32*1024)
		c.Status(200)
		for {
			n, err := file.Read(buf)
			if n > 0 {
				if _, werr := c.Writer.Write(buf[:n]); werr != nil {
					break
				}
				if flusher, ok := c.Writer.(http.Flusher); ok {
					flusher.Flush()
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}
	})
	r.Run(":8080")
}
