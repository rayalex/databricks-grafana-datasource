package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/jobs"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/rayalex/databricks/pkg/models"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &Datasource{
		settings: settings,
	}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	settings backend.DataSourceInstanceSettings
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct {
	RawQuery       string          `json:"queryText,omitempty"`
	ResourceType   string          `json:"resourceType"`
	ResourceParams json.RawMessage `json:"resourceParams,omitempty"`
}

const RESOURCE_TYPE_JOB_RUNS = "job_runs"

func (d *Datasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	fmt.Printf("Query Model: %v\n", qm)

	switch qm.ResourceType {
	case RESOURCE_TYPE_JOB_RUNS:
		return d.queryJobRuns(ctx, pCtx, query, qm)
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("Unknown resource kind: %s", qm.ResourceType))
	}
}

func (d *Datasource) queryJobRuns(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery, qm queryModel) backend.DataResponse {
	var response backend.DataResponse
	config, err := models.LoadPluginSettings(d.settings)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("Load plugin settings: %v", err))
	}

	dbxConfig := databricks.Config{
		Host:         config.Workspace,
		ClientID:     config.Secrets.ClientId,
		ClientSecret: config.Secrets.ClientSecret,
	}

	// TODO: Implement the query filtering + pagination
	w := databricks.Must(databricks.NewWorkspaceClient(&dbxConfig))
	all, err := w.Jobs.ListRunsAll(context.Background(), jobs.ListRunsRequest{
		Limit: 25,
	})

	// print some debug info on the response
	log.DefaultLogger.Info("Job Runs: %v\n", all)
	log.DefaultLogger.Info("Returned rows: %v\n", len(all))

	if err != nil {
		response.Error = err
		return response
	}
	// TODO: check if Start Time should be the first column
	frame := data.NewFrame("Databricks Job Runs",
		data.NewField("Job ID", nil, []int64{}),
		data.NewField("Run ID", nil, []int64{}),
		data.NewField("Attempt Number", nil, []int32{}),
		data.NewField("Status", nil, []string{}),
		data.NewField("Start Time", nil, []time.Time{}),
		data.NewField("End Time", nil, []time.Time{}),
		data.NewField("Queue Duration (seconds)", nil, []int64{}),
		data.NewField("Run Duration (seconds)", nil, []int64{}),
		data.NewField("Run URL", nil, []string{}),
	)

	for _, run := range all {
		frame.AppendRow(
			run.JobId,
			run.RunId,
			int32(run.AttemptNumber),
			string(run.Status.State),
			time.UnixMilli(run.StartTime),
			time.UnixMilli(run.EndTime),
			run.QueueDuration,
			run.RunDuration,
			run.RunPageUrl,
		)
	}

	response.Frames = append(response.Frames, frame)
	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	res := &backend.CheckHealthResult{}
	config, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)

	if err != nil {
		res.Status = backend.HealthStatusError
		res.Message = "Unable to load settings"
		return res, nil
	}

	if config.Secrets.ClientId == "" || config.Secrets.ClientSecret == "" {
		res.Status = backend.HealthStatusError
		res.Message = "Authentication is missing"
		return res, nil
	}

	dbxConfig := databricks.Config{
		Host:         config.Workspace,
		ClientID:     config.Secrets.ClientId,
		ClientSecret: config.Secrets.ClientSecret,
	}

	w := databricks.Must(databricks.NewWorkspaceClient(&dbxConfig))
	_, err = w.Jobs.ListAll(context.Background(), jobs.ListJobsRequest{
		Limit: 1,
	})

	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Error: %v", err),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}
