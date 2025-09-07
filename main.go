package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type URLRequest struct {
	URL string `json:"url" binding:"required,url"`
}

type URLResponse struct {
	ShortURL string `json:"short_url"`
}

type ShortLink struct {
	ShortID string `db:"short_id"`
	URL     string `db:"url"`
}

var db *sqlx.DB

func generateShortID(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")[:n]
}

func shortenURL(c *gin.Context) {
	var req URLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request " + err.Error()})
		return
	}

	// Генерируем уникальный shortID
	var shortID string
	for {
		shortID = generateShortID(6)
		var exists bool
		err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM short_links WHERE short_id=$1)", shortID)
		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !exists {
			break
		}
	}

	// Сохраняем в БД
	_, err := db.Exec("INSERT INTO short_links (short_id, url) VALUES ($1, $2)", shortID, req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save short link"})
		return
	}

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

	var link ShortLink
	err := db.Get(&link, "SELECT short_id, url FROM short_links WHERE short_id=$1", shortID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.HTML(http.StatusNotFound, "index.html", gin.H{
				"error": "Short URL not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}
	c.Redirect(http.StatusFound, link.URL)
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/shortlinker?sslmode=disable"
	}

	var err error
	db, err = sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalln("Failed to connect to DB:", err)
	}

	r := gin.Default()
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

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
