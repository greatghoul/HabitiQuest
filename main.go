package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			val = strings.Trim(val, `"'`)
			if key != "" {
				os.Setenv(key, val)
			}
		}
	}
}

//go:embed public/*
var staticFiles embed.FS

type HabiticaUser struct {
	Success bool `json:"success"`
	Data    struct {
		ID      string `json:"id"`
		Profile struct {
			Name string `json:"name"`
		} `json:"profile"`
		Stats struct {
			Hp  float64 `json:"hp"`
			Mp  float64 `json:"mp"`
			Exp float64 `json:"exp"`
			Lvl int     `json:"lvl"`
			Gp  float64 `json:"gp"`
		} `json:"stats"`
	} `json:"data"`
}

type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

type WebhookRequest struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	Label   string `json:"label"`
	Type    string `json:"type"`
	Options struct {
		QuestStarted  bool `json:"questStarted"`
		QuestFinished bool `json:"questFinished"`
		QuestInvited  bool `json:"questInvited"`
	} `json:"options"`
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	publicFS, _ := fs.Sub(staticFiles, "public")
	r.StaticFS("/static", http.FS(publicFS))

	r.GET("/", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("public/index.html")
		if err != nil {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	r.GET("/api/user", getUser)
	r.GET("/api/webhook/check", checkWebhook)
	r.POST("/api/webhook/configure", configureWebhook)
	r.POST("/api/webhook/event", handleWebhookEvent)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	r.Run(":" + port)
}

func habiticaRequest(method, path string, body io.Reader) (*http.Response, error) {
	userID := os.Getenv("HABITICA_USER_ID")
	apiToken := os.Getenv("HABITICA_API_TOKEN")
	req, _ := http.NewRequest(method, "https://habitica.com/api/v3"+path, body)
	req.Header.Set("x-client", fmt.Sprintf("%s-HabitiQuest", userID))
	req.Header.Set("x-api-user", userID)
	req.Header.Set("x-api-key", apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

func getCredentialsMessage() string {
	return "HABITICA_USER_ID or HABITICA_API_TOKEN not set. Get your credentials at https://habitica.com/user/settings/siteData , then configure them in Vercel: https://vercel.com/docs/projects/environment-variables"
}

func getUser(c *gin.Context) {
	userID := os.Getenv("HABITICA_USER_ID")
	apiToken := os.Getenv("HABITICA_API_TOKEN")

	if userID == "" || apiToken == "" {
		c.JSON(http.StatusOK, APIResponse{
			Success:   false,
			Message:   getCredentialsMessage(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	resp, err := habiticaRequest("GET", "/user", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, APIResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to reach Habitica API: %v", err),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var user HabiticaUser
	if err := json.Unmarshal(body, &user); err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success:   false,
			Message:   "Failed to parse Habitica response",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if !user.Success {
		c.JSON(http.StatusOK, APIResponse{
			Success:   false,
			Message:   getCredentialsMessage(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success:   true,
		Message:   fmt.Sprintf("Welcome, %s (Level %d)", user.Data.Profile.Name, user.Data.Stats.Lvl),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      user.Data,
	})
}

func checkWebhook(c *gin.Context) {
	userID := os.Getenv("HABITICA_USER_ID")
	apiToken := os.Getenv("HABITICA_API_TOKEN")

	if userID == "" || apiToken == "" {
		c.JSON(http.StatusOK, gin.H{"configured": false})
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	expectedURL := fmt.Sprintf("%s://%s/api/webhook/event", scheme, c.Request.Host)

	resp, err := habiticaRequest("GET", "/user/webhook", nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"configured": false})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var webhooks []struct {
		Enabled bool   `json:"enabled"`
		URL     string `json:"url"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(body, &webhooks); err != nil {
		c.JSON(http.StatusOK, gin.H{"configured": false})
		return
	}

	for _, w := range webhooks {
		if w.Enabled && w.URL == expectedURL && w.Type == "questActivity" {
			c.JSON(http.StatusOK, gin.H{"configured": true})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"configured": false})
}

func configureWebhook(c *gin.Context) {
	userID := os.Getenv("HABITICA_USER_ID")
	apiToken := os.Getenv("HABITICA_API_TOKEN")

	if userID == "" || apiToken == "" {
		c.JSON(http.StatusOK, APIResponse{
			Success:   false,
			Message:   getCredentialsMessage(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	webhookURL := fmt.Sprintf("%s://%s/api/webhook/event", scheme, c.Request.Host)
	fmt.Printf("[Webhook] Configuring webhook URL: %s\n", webhookURL)

	body := WebhookRequest{
		Enabled: true,
		URL:     webhookURL,
		Label:   "HabitiQuest Quest Webhook",
		Type:    "questActivity",
	}
	body.Options.QuestStarted = false
	body.Options.QuestFinished = false
	body.Options.QuestInvited = true

	jsonBody, _ := json.Marshal(body)
	fmt.Printf("[Webhook] Request body: %s\n", string(jsonBody))

	resp, err := habiticaRequest("POST", "/user/webhook", bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Printf("[Webhook] HTTP error: %v\n", err)
		c.JSON(http.StatusBadGateway, APIResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to configure webhook: %v", err),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("[Webhook] Response status: %d, body: %s\n", resp.StatusCode, string(respBody))

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID string `json:"id"`
		} `json:"data"`
		Err  string `json:"err"`
		ErrV struct {
			Message string `json:"message"`
		} `json:"errV"`
	}
	json.Unmarshal(respBody, &result)

	if !result.Success {
		errMsg := result.Err
		if result.ErrV.Message != "" {
			errMsg = result.ErrV.Message
		}
		fmt.Printf("[Webhook] Habitica API error: %s\n", errMsg)
		c.JSON(http.StatusOK, APIResponse{
			Success:   false,
			Message:   fmt.Sprintf("Habitica API error: %s", errMsg),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success:   true,
		Message:   fmt.Sprintf("Webhook configured successfully! ID: %s", result.Data.ID),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func handleWebhookEvent(c *gin.Context) {
	fmt.Printf("[Webhook Event] Query params: %s\n", c.Request.URL.RawQuery)
	fmt.Printf("[Webhook Event] Headers:\n")
	for k, v := range c.Request.Header {
		fmt.Printf("  %s: %s\n", k, v)
	}
	body, _ := io.ReadAll(c.Request.Body)
	fmt.Printf("[Webhook Event] Body: %s\n", string(body))
	c.JSON(http.StatusOK, gin.H{"success": true})
}
