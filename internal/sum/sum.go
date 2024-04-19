package sum

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"google.golang.org/api/calendar/v3"
)

// ThisYear collects the user's events in this year and summarizes them.
func ThisYear(ctx context.Context, srv *calendar.Service) error {
	now := time.Now()
	timeMin := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
	timeMax := now

	var pageToken string
	sums := make(map[string]time.Duration)
	for {
		q := srv.Events.List("primary").ShowDeleted(false).
			SingleEvents(true).
			TimeMin(timeMin.Format(time.RFC3339)).
			TimeMax(timeMax.Format(time.RFC3339)).
			MaxResults(2500).
			OrderBy("startTime").
			Context(ctx)

		if pageToken != "" {
			q.PageToken(pageToken)
		}

		slog.InfoContext(ctx, "Requesting events", "PageToken", pageToken)
		events, err := q.Do()
		if err != nil {
			return fmt.Errorf("retrieve the user's events: %w", err)
		}
		slog.InfoContext(ctx, "Retrieved events", "Total", len(events.Items), "NextPageToken", events.NextPageToken)

		for _, item := range events.Items {
			if item.Start.DateTime == "" || item.End.DateTime == "" {
				continue
			}
			start, _ := time.Parse(time.RFC3339, item.Start.DateTime)
			end, _ := time.Parse(time.RFC3339, item.End.DateTime)
			sums[item.Summary] += end.Sub(start)
		}

		if events.NextPageToken != "" {
			pageToken = events.NextPageToken
			continue
		}
		break
	}

	type result struct {
		Summary  string
		Duration time.Duration
	}
	results := make([]result, 0, len(sums))
	for summary, duration := range sums {
		results = append(results, result{summary, duration})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Duration > results[j].Duration
	})

	slog.InfoContext(ctx, "Collected results", "Total", len(results))
	max := 30
	if len(results) > max {
		results = results[:max]
		slog.InfoContext(ctx, "Truncating results", "Total", len(results))
	}

	for i, r := range results {
		fmt.Printf("%d. %s: %v\n", i+1, r.Summary, r.Duration)
	}
	return nil
}
