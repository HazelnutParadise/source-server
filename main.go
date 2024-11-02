package main

import (
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
		// 取得本地文件
		filePath := filepath.Join(sourceDir, source)
		file, err := os.Open(filePath)
		if err != nil {
			c.JSON(404, gin.H{"message": "file not found"})
			return
		}
		defer file.Close()
		c.File(file.Name())
	})
	r.Run(":8080")
}
