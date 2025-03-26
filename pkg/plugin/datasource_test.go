package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestQueryData(t *testing.T) {
	ds := Datasource{}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A"},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}

func TestBuildListRunsRequest(t *testing.T) {
	t.Parallel()

	t.Run("should return valid request when no params are used", func(t *testing.T) {
		params := jobRunParams{}
		query := backend.DataQuery{}

		_, err := buildListRunsRequest(params, query)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("should pass valid job id", func(t *testing.T) {
		params := jobRunParams{
			JobID: "123",
		}

		query := backend.DataQuery{}

		req, err := buildListRunsRequest(params, query)
		if err != nil {
			t.Error(err)
		}

		if req.JobId != 123 {
			t.Errorf("expected job id to be 123, got %d", req.JobId)
		}
	})

	t.Run("should set active and completed", func(t *testing.T) {
		params := jobRunParams{
			ActiveOnly:    true,
			CompletedOnly: true,
		}

		query := backend.DataQuery{}

		req, err := buildListRunsRequest(params, query)
		if err != nil {
			t.Error(err)
		}

		if !req.ActiveOnly {
			t.Error("expected active only to be true")
		}

		if !req.CompletedOnly {
			t.Error("expected completed only to be true")
		}
	})

	t.Run("should return error when job id is invalid", func(t *testing.T) {
		params := jobRunParams{
			JobID: "invalid",
		}

		query := backend.DataQuery{}

		_, err := buildListRunsRequest(params, query)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("should set time range", func(t *testing.T) {
		params := jobRunParams{}
		query := backend.DataQuery{
			TimeRange: backend.TimeRange{
				From: time.Now(),
				To:   time.Now(),
			},
		}

		req, err := buildListRunsRequest(params, query)
		if err != nil {
			t.Error(err)
		}

		if req.StartTimeFrom != query.TimeRange.From.UnixMilli() {
			t.Error("expected start time from to be set")
		}

		if req.StartTimeTo != query.TimeRange.To.UnixMilli() {
			t.Error("expected start time to to be set")
		}
	})
}
