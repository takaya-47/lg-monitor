package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func main() {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	url := "https://www.city.uki.kumamoto.jp/"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		fmt.Println("error when creating request: ", err)
		return
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("error when sending request: ", err)
		return
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Println("error: status code is not OK: ", res.StatusCode)
		return
	}

	fmt.Println("success: status code is OK", res.StatusCode)
}
