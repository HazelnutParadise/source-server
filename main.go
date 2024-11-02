package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 從 docker compose volume 取得 source 檔案路徑
var sourceDir = os.Getenv("SOURCE_DIR")

func main() {
	// 如果 sourceDir 不存在，則創建
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		os.MkdirAll(sourceDir, 0755)
	}
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Origin"},
	}))
	r.GET("/:source", func(c *gin.Context) {
		source := c.Param("source")
		filePath := filepath.Join(sourceDir, source)

		file, err := os.Open(filePath)
		if err != nil {
			c.JSON(404, gin.H{"message": "file not found"})
			return
		}
		defer file.Close()

		// 獲取檔案資訊
		fileInfo, err := file.Stat()
		if err != nil {
			c.JSON(500, gin.H{"message": "could not get file info"})
			return
		}

		// 設置響應標頭
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", "attachment; filename="+source)
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// 使用串流傳輸
		c.Stream(func(w io.Writer) bool {
			_, err := io.Copy(w, file)
			return err == nil
		})
	})
	r.Run(":8080")
}
