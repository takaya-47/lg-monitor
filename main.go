package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := http.Client{
		Timeout: 10 * time.Second,
	}
	url := "https://www.city.uki.kumamoto.jp/"

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Received interrupt signal, bye!")
			return
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				fmt.Println("error when creating request: ", err)
				continue
			}

			res, err := client.Do(req)
			if err != nil {
				fmt.Println("error when sending request: ", err)
				continue
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				fmt.Println("error: status code is not OK: ", res.StatusCode)
				continue
			}

			fmt.Println("success: status code is OK", res.StatusCode)
		}
	}
}
