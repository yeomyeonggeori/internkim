package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: download <url> <output-path>")
		os.Exit(1)
	}

	url := os.Args[1]
	output := os.Args[2]

	client := &http.Client{Timeout: 30 * time.Second}
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("User-Agent", "internkim/1.0 (https://example.test; iam@example.test) Go-http-client/1.1")
	request.Header.Set("Accept", "*/*")

	response, err := client.Do(request)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		fmt.Printf("ERROR: HTTP %d\n", response.StatusCode)
		os.Exit(1)
	}

	contentType := response.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		fmt.Println("ERROR: got HTML instead of file")
		os.Exit(1)
	}

	file, err := os.Create(output)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	written, err := io.Copy(file, response.Body)
	if err != nil {
		os.Remove(output)
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("OK %d %s\n", written, contentType)
}
