package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"log/slog"

	"github.com/hareku/google-calendar-sum/internal/sum"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func init() {
	level := slog.LevelInfo
	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}),
	))
	slog.Info("Logger initialized", slog.String("level", level.String()))
}

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(ctx context.Context) error {
	srv, err := newCalendarService(ctx)
	if err != nil {
		return fmt.Errorf("initialize calendar service: %w", err)
	}
	if err := sum.ThisYear(ctx, srv); err != nil {
		return fmt.Errorf("summarize events: %w", err)
	}
	return nil
}

func newCalendarService(ctx context.Context) (*calendar.Service, error) {
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("read client secret file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parse client secret file to config: %w", err)
	}
	client, err := getClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("get http client: %w", err)
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("retrieve Calendar client: %w", err)
	}

	return srv, nil
}

func getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err == nil {
		return config.Client(ctx, tok), nil
	}
	slog.InfoContext(ctx, "Saved token not found", "error", err)

	tok, err = getTokenFromWeb(ctx, config)
	if err == nil {
		saveToken(tokFile, tok)
		return config.Client(ctx, tok), nil
	}
	slog.InfoContext(ctx, "Failed to get token from web", "error", err)

	return nil, errors.New("no token available")
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("read authorization code: %w", err)
	}

	tok, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	return tok, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
