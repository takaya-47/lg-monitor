package main

import (
	"context"
	"errors"
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
			err := request(ctx, client, url)
			if err != nil {
				fmt.Println("Error:", err)
			}
		}
	}
}

func request(ctx context.Context, client http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.New("error when creating request: " + err.Error())
	}

	res, err := client.Do(req)
	if err != nil {
		return errors.New("error when sending request: " + err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status code is not OK: %d", res.StatusCode)
	}

	fmt.Println("success: status code is OK", res.StatusCode)
	return nil
}
