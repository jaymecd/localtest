package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/parnurzeal/gorequest"
)

const remoteURL = "https://whoami.local.test"

var version string
var hostname string

func init() {
	version = strings.TrimPrefix(runtime.Version(), "go")
	hostname, _ = os.Hostname()
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Server is running on port http://localhost:8080 ...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func fetchWithHttp(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	data, err := parseResponse(body)
	if err != nil {
		return "", err
	}

	return data, nil
}

func fetchWithResty(url string) (string, error) {
	client := resty.New()
	resp, err := client.R().Get(url)
	if err != nil {
		return "", err
	}

	if resp.IsError() {
		return "", fmt.Errorf("received non-OK HTTP status: %d", resp.StatusCode())
	}

	data, err := parseResponse(resp.Body())
	if err != nil {
		return "", err
	}

	return data, nil
}

func fetchWithGoRequests(url string) (string, error) {
	request := gorequest.New()
	resp, body, errs := request.Get(url).EndBytes()
	if len(errs) > 0 {
		return "", fmt.Errorf("error fetching with GoRequest: %v", errs)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("received non-OK HTTP status: %d", resp.StatusCode)
	}

	data, err := parseResponse(body)
	if err != nil {
		return "", err
	}

	return data, nil
}

func parseResponse(body []byte) (string, error) {
	var matches []string

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if expectedLine(line) {
			matches = append(matches, indentLine(line))
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no matching lines found")
	}

	return strings.Join(matches, "\n"), nil
}

func expectedLine(line string) bool {
	if strings.Contains(line, "Hostname") {
		return true
	}

	if strings.Contains(line, "IP") && strings.Contains(line, ".") && !strings.Contains(line, "127.0.0.1") {
		return true
	}

	return false
}

func indentLine(line string) string {
	return "    " + line
}

func handler(w http.ResponseWriter, r *http.Request) {
	currentTime := time.Now()

	results := []string{
		"Hi, I'm GoLang/" + version + " service running on '" + hostname + "' host.",
		"",
		"Time is " + currentTime.Format(time.RFC3339),
		"",
		"Rendering " + remoteURL + " page",
	}

	fetchFuncsToTry := []struct {
		name string
		exec func(string) (string, error)
	}{
		{"HTTP", fetchWithHttp},
		{"Resty", fetchWithResty},
		{"GoRequest", fetchWithGoRequests},
	}

	for i, fn := range fetchFuncsToTry {
		results = append(results, fmt.Sprintf("\nrequest (%d) - using '%s' lib:\n", i+1, fn.name))

		data, err := fn.exec(remoteURL)
		if err != nil {
			results = append(results, indentLine(fmt.Sprintf("# Error: %v", err)))
			continue
		}
		results = append(results, data)
	}

	results = append(results, "\nThank you!")

	// Combine and send all results
	w.Header().Set("Content-Type", "text/plain")
	for _, result := range results {
		fmt.Fprintln(w, result)
	}
}
