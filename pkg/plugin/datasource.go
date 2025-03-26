package plugin

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
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

const (
	resourceTypeJobRuns         = "job_runs"
	resourceTypePipelineUpdates = "pipeline_updates"
	jobRunPageLimit             = 25
	pipelineUpdatePageLimit     = 25
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

type jobRunParams struct {
	JobID         string `json:"jobId,omitempty"`
	ActiveOnly    bool   `json:"activeOnly,omitempty"`
	CompletedOnly bool   `json:"completedOnly,omitempty"`
	RunType       string `json:"runType,omitempty"`
}

func (d *Datasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	switch qm.ResourceType {
	case resourceTypeJobRuns:
		return d.queryJobRuns(ctx, pCtx, query, qm)
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unknown resource kind: %s", qm.ResourceType))
	}
}

func parseJobRunParams(query backend.DataQuery, qm queryModel) (jobRunParams, error) {
	var params jobRunParams
	if qm.ResourceParams != nil {
		if err := json.Unmarshal(qm.ResourceParams, &params); err != nil {
			return params, fmt.Errorf("failed to unmarshal query params: %v", err)
		}
	}

	return params, nil
}

func buildListRunsRequest(params jobRunParams, query backend.DataQuery) (jobs.ListRunsRequest, error) {
	req := jobs.ListRunsRequest{
		Limit:         25, // 25 is the max limit for this API - per page
		ActiveOnly:    params.ActiveOnly,
		CompletedOnly: params.CompletedOnly,
	}

	// apply job id filter, if set
	if params.JobID != "" {
		jobId, err := strconv.ParseInt(params.JobID, 10, 64)
		if err != nil {
			return req, err
		}

		req.JobId = jobId
	}

	// apply time range filter, if set
	if !query.TimeRange.From.IsZero() && !query.TimeRange.To.IsZero() {
		req.StartTimeFrom = query.TimeRange.From.UnixMilli()
		req.StartTimeTo = query.TimeRange.To.UnixMilli()
	}

	// apply run type filter
	if params.RunType != "" {
		req.RunType = jobs.RunType(params.RunType)
	}

	return req, nil
}

func builtJobRunFrame(runs []jobs.BaseRun) *data.Frame {
	frame := data.NewFrame("Databricks Job Runs",
		data.NewField("Start Time", nil, []time.Time{}),
		data.NewField("End Time", nil, []time.Time{}),
		data.NewField("Job ID", nil, []string{}),
		data.NewField("Run ID", nil, []string{}),
		data.NewField("Run Name", nil, []string{}),
		data.NewField("Description", nil, []string{}),
		data.NewField("Attempt Number", nil, []int32{}),
		data.NewField("Status", nil, []string{}),
		data.NewField("Queue Duration (seconds)", nil, []int64{}),
		data.NewField("Run Duration (seconds)", nil, []int64{}),
		data.NewField("Run URL", nil, []string{}),
	)

	// sort rows ascending by StartTime
	slices.SortFunc(runs, func(i, j jobs.BaseRun) int {
		return cmp.Compare(i.StartTime, j.StartTime)
	})

	for _, run := range runs {
		frame.AppendRow(
			time.UnixMilli(run.StartTime),
			time.UnixMilli(run.EndTime),
			fmt.Sprintf("%d", run.JobId),
			fmt.Sprintf("%d", run.RunId),
			run.RunName,
			run.Description,
			int32(run.AttemptNumber),
			string(run.Status.State),
			run.QueueDuration,
			run.RunDuration,
			run.RunPageUrl,
		)
	}

	return frame
}

func (d *Datasource) queryJobRuns(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery, qm queryModel) backend.DataResponse {
	params, err := parseJobRunParams(query, qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to parse query params: %v", err))
	}

	w, err := d.getDatabricksClient(ctx, pCtx)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("failed to get databricks client: %v", err))
	}

	request, err := buildListRunsRequest(params, query)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to build list runs request: %v", err))
	}

	log.DefaultLogger.Info("Querying job runs",
		"limit", request.Limit,
		"start_time_from", time.UnixMilli(request.StartTimeFrom),
		"start_time_to", time.UnixMilli(request.StartTimeTo),
		"job_id", request.JobId,
		"active_only", request.ActiveOnly,
		"completed_only", request.CompletedOnly,
		"run_type", request.RunType,
	)

	jobsService := &workspaceClientWrapper{client: w}
	jobRuns, err := fetchJobRuns(ctx, jobsService, request, jobRunPageLimit)

	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("failed to fetch job runs: %v", err))
	}

	var response backend.DataResponse
	frame := builtJobRunFrame(jobRuns)
	response.Frames = append(response.Frames, frame)
	return response
}

func fetchJobRuns(ctx context.Context, client DatabricksJobsService, request jobs.ListRunsRequest, maxItems int) ([]jobs.BaseRun, error) {
	iter := client.ListRuns(ctx, request)
	return fetchWithLimit(ctx, iter, maxItems)
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

	w, _ := d.getDatabricksClient(context.Background(), req.PluginContext)
	_, err = w.CurrentUser.Me(context.Background())

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

func (d *Datasource) getDatabricksClient(ctx context.Context, pCtx backend.PluginContext) (*databricks.WorkspaceClient, error) {
	config, err := models.LoadPluginSettings(d.settings)
	if err != nil {
		return nil, fmt.Errorf("load plugin settings: %v", err)
	}

	dbxConfig := databricks.Config{
		Host:         config.Workspace,
		ClientID:     config.Secrets.ClientId,
		ClientSecret: config.Secrets.ClientSecret,
	}

	return databricks.Must(databricks.NewWorkspaceClient(&dbxConfig)), nil
}
