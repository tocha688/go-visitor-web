package main

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

//go:embed templates
var templateFS embed.FS

func obfuscateHTML(html string) string {
	if len(html) == 0 {
		return html
	}
	obfuscated := strings.ReplaceAll(html, "\n", "")
	obfuscated = strings.ReplaceAll(obfuscated, "\t", "")
	obfuscated = strings.ReplaceAll(obfuscated, "  ", " ")
	obfuscated = strings.ReplaceAll(obfuscated, " >", ">")
	obfuscated = strings.ReplaceAll(obfuscated, "> <", "><")
	if len(obfuscated) < 2 {
		return obfuscated
	}
	for i := 0; i < 3; i++ {
		comment := fmt.Sprintf("<!--%d-->", rand.Intn(999999))
		pos := rand.Intn(len(obfuscated)-2) + 1
		obfuscated = obfuscated[:pos] + comment + obfuscated[pos:]
	}
	return obfuscated
}

type obfuscatingWriter struct {
	gin.ResponseWriter
	body *strings.Builder
}

func (w *obfuscatingWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return len(b), nil
}

func htmlObfuscator() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/" || c.Request.URL.Path == "/loading" {
			wrapper := &obfuscatingWriter{
				ResponseWriter: c.Writer,
				body:           &strings.Builder{},
			}
			c.Writer = wrapper
			c.Next()
			if wrapper.body.Len() > 0 {
				obfuscated := obfuscateHTML(wrapper.body.String())
				c.Header("Content-Length", fmt.Sprintf("%d", len(obfuscated)))
				c.Writer.Write([]byte(obfuscated))
			}
		} else {
			c.Next()
		}
	}
}

type Config struct {
	App   AppConfig   `yaml:"app"`
	Stats StatsConfig `yaml:"stats"`
}

type AppConfig struct {
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	AdminPassword string `yaml:"admin_password"`
	TargetURL     string `yaml:"target_url"`
}

type StatsConfig struct {
	VisitFile string `yaml:"visit_file"`
}

type HistoryEntry struct {
	Count     int            `json:"count"`
	UniqueIPs map[string]int `json:"unique_ips"`
	Crawlers  int            `json:"crawlers"`
}

type VisitStats struct {
	RealUsers    int                     `json:"real_users"`
	Crawlers     int                     `json:"crawlers"`
	History      map[string]HistoryEntry `json:"history"`
	UniqueIPs    map[string]int          `json:"unique_ips"`
	Logs         []VisitLog              `json:"logs"`
	Fingerprints map[string]Fingerprint  `json:"fingerprints"`
}

type Fingerprint struct {
	IP          string   `json:"ip"`
	Country     string   `json:"country"`
	UserAgent   string   `json:"user_agent"`
	Screen      string   `json:"screen"`
	Timezone    string   `json:"timezone"`
	Language    string   `json:"language"`
	Platform    string   `json:"platform"`
	CanvasHash  string   `json:"canvas_hash"`
	WebGL       string   `json:"webgl"`
	Cookies     bool     `json:"cookies"`
	Touch       bool     `json:"touch"`
	ColorDepth  int      `json:"color_depth"`
	PixelRatio  float64  `json:"pixel_ratio"`
	Fonts       []string `json:"fonts"`
	Plugins     string   `json:"plugins"`
	CanvasFonts string   `json:"canvas_fonts"`
	JA3         string   `json:"ja3"`
	JA4         string   `json:"ja4"`
	Akamai      string   `json:"akamai"`
	TLSRaw      string   `json:"tls_raw"`
}

type VisitLog struct {
	Timestamp     string            `json:"timestamp"`
	IP            string            `json:"ip"`
	Country       string            `json:"country"`
	UserAgent     string            `json:"user_agent"`
	IsCrawler     bool              `json:"is_crawler"`
	Headers       map[string]string `json:"headers"`
	Path          string            `json:"path"`
	Fingerprint   Fingerprint       `json:"fingerprint"`
	FingerprintID string            `json:"fingerprint_id"`
}

var config Config
var stats VisitStats

var ipCountryCache = make(map[string]string)
var ipCacheMutex sync.RWMutex

func getCountryByIP(ip string) string {
	if ip == "" {
		return "Unknown"
	}
	ipCacheMutex.RLock()
	if country, ok := ipCountryCache[ip]; ok {
		ipCacheMutex.RUnlock()
		return country
	}
	ipCacheMutex.RUnlock()

	tryAPIs := []string{
		fmt.Sprintf("https://ipapi.co/%s/json/", ip),
		fmt.Sprintf("http://ip-api.com/json/%s", ip),
	}

	for _, api := range tryAPIs {
		resp, err := http.Get(api)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			continue
		}

		var country string
		if c, ok := result["country_name"].(string); ok && c != "" {
			country = c
		} else if c, ok := result["country"].(string); ok && c != "" {
			country = c
		} else if c, ok := result["countryCode"].(string); ok && c != "" {
			country = c
		}

		if country != "" {
			ipCacheMutex.Lock()
			ipCountryCache[ip] = country
			ipCacheMutex.Unlock()
			return country
		}
	}

	return "Unknown"
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	loadConfig()
	loadStats()

	r := gin.Default()

	tmpl, err := template.ParseFS(templateFS, "templates/*")
	if err != nil {
		panic(err)
	}
	r.SetHTMLTemplate(tmpl)

	// r.Use(htmlObfuscator()) // disabled

	r.GET("/", showIndex)
	r.POST("/api/visit", recordVisitAPI)
	r.GET("/loading", processVisit)
	r.GET("/adm", adminLoginPage)
	r.POST("/adm/login", adminLogin)
	r.POST("/adm/logout", adminLogout)
	r.GET("/adm/dashboard", adminDashboard)
	r.POST("/adm/reset", adminReset)
	r.POST("/adm/clearLogs", adminClearLogs)
	r.POST("/adm/updateTarget", adminUpdateTarget)
	r.POST("/adm/updatePassword", adminUpdatePassword)
	r.GET("/api/stats", apiStats)
	r.NoRoute(showIndex)

	if err := os.MkdirAll(filepath.Dir(config.Stats.VisitFile), 0755); err != nil {
		panic(err)
	}

	fmt.Printf("Server running at http://%s:%d\n", config.App.Host, config.App.Port)
	r.Run(fmt.Sprintf("%s:%d", config.App.Host, config.App.Port))
}

func loadConfig() {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		config = Config{
			App: AppConfig{
				Host:          "0.0.0.0",
				Port:          8080,
				AdminPassword: "123456",
				TargetURL:     "https://www.example.com",
			},
			Stats: StatsConfig{
				VisitFile: "data/visits.json",
			},
		}
		return
	}
	yaml.Unmarshal(data, &config)
	if config.Stats.VisitFile == "" {
		config.Stats.VisitFile = "data/visits.json"
	}
}

func loadStats() {
	stats = VisitStats{
		History:      make(map[string]HistoryEntry),
		UniqueIPs:    make(map[string]int),
		Logs:         []VisitLog{},
		Fingerprints: make(map[string]Fingerprint),
	}

	data, err := os.ReadFile(config.Stats.VisitFile)
	if err != nil || len(data) == 0 {
		return
	}

	json.Unmarshal(data, &stats)

	if stats.History == nil {
		stats.History = make(map[string]HistoryEntry)
	}
	if stats.UniqueIPs == nil {
		stats.UniqueIPs = make(map[string]int)
	}
	if stats.Logs == nil {
		stats.Logs = []VisitLog{}
	}
	if stats.Fingerprints == nil {
		stats.Fingerprints = make(map[string]Fingerprint)
	}
	for date, entry := range stats.History {
		if entry.UniqueIPs == nil {
			entry.UniqueIPs = make(map[string]int)
			stats.History[date] = entry
		}
		if entry.Crawlers < 0 {
			entry.Crawlers = 0
			stats.History[date] = entry
		}
	}
}

func saveStats() {
	data, _ := json.MarshalIndent(stats, "", "  ")
	os.WriteFile(config.Stats.VisitFile, data, 0644)
}

func isCrawlerByUA(ua string) bool {
	if ua == "" {
		return true
	}

	uaLower := strings.ToLower(ua)
	crawlers := []string{"bot", "spider", "crawl", "googlebot", "bingbot", "slurp", "duckduckbot", "baiduspider", "yandexbot", "sogou", "crawler", "scraper", "phantom", "headless", "puppeteer", "selenium", "playwright", "axios", "node-fetch", "python-requests", "java/", "go-http", "curl", "wget", "libwww", "httpclient"}
	for _, crawler := range crawlers {
		if strings.Contains(uaLower, crawler) {
			return true
		}
	}

	if strings.Contains(uaLower, "mozilla/") && !strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "firefox") && !strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "edge") && !strings.Contains(uaLower, "opera") {
		return true
	}

	if strings.Contains(uaLower, "chrome/") && strings.Contains(uaLower, "headless") {
		return true
	}

	return false
}

func getClientIP(c *gin.Context) string {
	ip := c.GetHeader("X-Forwarded-For")
	if ip != "" {
		return strings.Split(ip, ",")[0]
	}
	ip = c.GetHeader("X-Real-IP")
	if ip != "" {
		return ip
	}
	return c.ClientIP()
}

func recordVisitAPI(c *gin.Context) {
	var req struct {
		Fingerprint Fingerprint `json:"fingerprint"`
		Path        string      `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ua := c.GetHeader("User-Agent")
	isCrawler := isCrawlerByUA(ua)

	if req.Fingerprint.IP == "" || req.Fingerprint.IP == "Unknown" {
		req.Fingerprint.IP = getClientIP(c)
	}
	if req.Fingerprint.UserAgent == "" {
		req.Fingerprint.UserAgent = ua
	}

	visitPath := req.Path
	if visitPath == "" {
		visitPath = "/"
	}

	recordVisitWithFingerprint(c, visitPath, "", ua, isCrawler, req.Fingerprint)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func showIndex(c *gin.Context) {
	path := c.Request.URL.Path
	ua := c.GetHeader("User-Agent")
	isCrawler := isCrawlerByUA(ua)

	if isCrawler {
		ip := getClientIP(c)
		recordVisitWithFingerprint(c, path, ip, ua, isCrawler, Fingerprint{})
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Delay": 3,
		"Path":  path,
	})
}

func recordVisit(c *gin.Context, path, ip, ua string, isCrawler bool) {
	recordVisitWithFingerprint(c, path, ip, ua, isCrawler, Fingerprint{})
}

func recordVisitWithFingerprint(c *gin.Context, path, ip, ua string, isCrawler bool, fp Fingerprint) {
	if ip == "" {
		ip = getClientIP(c)
	}
	if ua == "" {
		ua = c.GetHeader("User-Agent")
	}
	if path == "" {
		path = "/"
	}

	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if fp.IP == "" {
		fp.IP = ip
	}
	if fp.UserAgent == "" {
		fp.UserAgent = ua
	}

	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s", fp.UserAgent, fp.Screen, fp.Timezone, fp.Platform, fp.CanvasHash, fp.WebGL)
	fpID := fmt.Sprintf("%x", sha256.Sum256([]byte(data)))[:16]

	log := VisitLog{
		Timestamp:     time.Now().Format("2006-01-02 15:04:05"),
		IP:            fp.IP,
		Country:       fp.Country,
		UserAgent:     fp.UserAgent,
		IsCrawler:     isCrawler,
		Headers:       headers,
		Path:          path,
		Fingerprint:   fp,
		FingerprintID: fpID,
	}

	stats.Logs = append([]VisitLog{log}, stats.Logs...)
	if len(stats.Logs) > 1000 {
		stats.Logs = stats.Logs[:1000]
	}

	date := time.Now().Format("2006-01-02")
	if stats.History[date].UniqueIPs == nil {
		stats.History[date] = HistoryEntry{UniqueIPs: make(map[string]int)}
	}
	entry := stats.History[date]
	entry.Count++
	if _, exists := entry.UniqueIPs[fp.IP]; !exists {
		entry.UniqueIPs[fp.IP] = 1
		if isCrawler {
			entry.Crawlers++
		}
	}
	stats.History[date] = entry

	if _, exists := stats.UniqueIPs[fp.IP]; !exists {
		stats.UniqueIPs[fp.IP] = 1
		if isCrawler {
			stats.Crawlers++
		} else {
			stats.RealUsers++
		}
	}

	stats.Fingerprints[fpID] = fp
	saveStats()
}

func processVisit(c *gin.Context) {
	from := c.DefaultQuery("from", "/")
	c.HTML(http.StatusOK, "loading.html", gin.H{
		"TargetURL": config.App.TargetURL,
		"Delay":     2,
		"Path":      from,
	})
}

func adminLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

func adminLogin(c *gin.Context) {
	password := c.PostForm("password")
	if password == config.App.AdminPassword {
		c.SetCookie("admin", "true", 3600, "/", "", false, true)
		c.Redirect(http.StatusFound, "/adm/dashboard")
		return
	}
	c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "密码错误"})
}

func adminLogout(c *gin.Context) {
	c.SetCookie("admin", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/adm")
}

func adminDashboard(c *gin.Context) {
	admin, _ := c.Cookie("admin")
	if admin != "true" {
		c.Redirect(http.StatusFound, "/adm")
		return
	}

	var history []struct {
		Date      string
		Count     int
		UniqueIPs int
		Crawlers  int
	}
	for date, entry := range stats.History {
		history = append(history, struct {
			Date      string
			Count     int
			UniqueIPs int
			Crawlers  int
		}{date, entry.Count, len(entry.UniqueIPs), entry.Crawlers})
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"RealUsers": len(stats.UniqueIPs),
		"Crawlers":  stats.Crawlers,
		"UniqueIPs": len(stats.UniqueIPs),
		"History":   history,
		"Logs":      stats.Logs,
		"TargetURL": config.App.TargetURL,
	})
}

func apiStats(c *gin.Context) {
	admin, _ := c.Cookie("admin")
	if admin != "true" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var history []struct {
		Date      string
		Count     int
		UniqueIPs int
		Crawlers  int
	}
	for date, entry := range stats.History {
		history = append(history, struct {
			Date      string
			Count     int
			UniqueIPs int
			Crawlers  int
		}{date, entry.Count, len(entry.UniqueIPs), entry.Crawlers})
	}

	var userLogs []VisitLog
	for _, log := range stats.Logs {
		if !log.IsCrawler {
			userLogs = append(userLogs, log)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"RealUsers": len(stats.UniqueIPs),
		"Crawlers":  stats.Crawlers,
		"UniqueIPs": len(stats.UniqueIPs),
		"History":   history,
		"Logs":      userLogs,
		"TargetURL": config.App.TargetURL,
	})
}

func adminReset(c *gin.Context) {
	admin, _ := c.Cookie("admin")
	if admin != "true" {
		c.Redirect(http.StatusFound, "/adm")
		return
	}

	stats.RealUsers = 0
	stats.Crawlers = 0
	stats.History = make(map[string]HistoryEntry)
	stats.UniqueIPs = make(map[string]int)
	stats.Logs = []VisitLog{}
	stats.Fingerprints = make(map[string]Fingerprint)
	saveStats()
	c.Redirect(http.StatusFound, "/adm/dashboard")
}

func adminClearLogs(c *gin.Context) {
	admin, _ := c.Cookie("admin")
	if admin != "true" {
		c.Redirect(http.StatusFound, "/adm")
		return
	}

	stats.Logs = []VisitLog{}
	saveStats()
	c.Redirect(http.StatusFound, "/adm/dashboard")
}

func adminUpdateTarget(c *gin.Context) {
	admin, _ := c.Cookie("admin")
	if admin != "true" {
		c.Redirect(http.StatusFound, "/adm")
		return
	}

	targetURL := c.PostForm("target_url")
	if targetURL != "" {
		config.App.TargetURL = targetURL
		data, _ := yaml.Marshal(config)
		os.WriteFile("config.yaml", data, 0644)
	}
	c.Redirect(http.StatusFound, "/adm/dashboard")
}

func adminUpdatePassword(c *gin.Context) {
	admin, _ := c.Cookie("admin")
	if admin != "true" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	password := c.PostForm("password")
	if password != "" {
		config.App.AdminPassword = password
		data, _ := yaml.Marshal(config)
		os.WriteFile("config.yaml", data, 0644)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "password required"})
}
