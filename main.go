package main

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type URLRequest struct {
	URL string `json:"url" binding:"required,url"`
}

type URLResponse struct {
	ShortURL string `json:"short_url"`
}

var (
	urlStore = make(map[string]string)
	mu       sync.RWMutex
)

func generateShortID(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)[:n]
}

func shortenURL(c *gin.Context) {
	var req URLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request " + err.Error()})
		return
	}

	shortID := generateShortID(6)

	mu.Lock()
	for {
		if _, exists := urlStore[shortID]; !exists {
			break
		}
		shortID = generateShortID(6)
	}
	urlStore[shortID] = req.URL
	mu.Unlock()

	host := c.Request.Host
	scheme := "http://"
	if c.Request.TLS != nil {
		scheme = "https://"
	}

	c.JSON(http.StatusOK, URLResponse{
		ShortURL: scheme + host + "/" + shortID,
	})
}

func redirect(c *gin.Context) {
	shortID := c.Param("short")

	mu.RLock()
	url, exists := urlStore[shortID]
	mu.RUnlock()

	if !exists {
		c.HTML(http.StatusNotFound, "index.html", gin.H{
			"error": "Short URL not found",
		})
		return
	}
	c.Redirect(http.StatusFound, url)
}

func main() {
	r := gin.Default()

	// Статика
	r.Static("/static", "./static")

	// HTML-шаблоны
	r.LoadHTMLGlob("templates/*")

	// Главная страница
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Версионированный API
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			v1.POST("/shorten", shortenURL)
		}
	}

	r.GET("/:short", redirect)

	r.Run(":8080")
}
