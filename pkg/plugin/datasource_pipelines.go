package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/databricks/databricks-sdk-go/service/pipelines"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type pipelineParams struct {
	Filter string `json:"filter,omitempty"`
}

func parsePipelineParams(_ backend.DataQuery, qm queryModel) (pipelineParams, error) {
	var params pipelineParams
	if qm.ResourceParams != nil {
		if err := json.Unmarshal(qm.ResourceParams, &params); err != nil {
			return params, fmt.Errorf("failed to unmarshal query params: %v", err)
		}
	}

	return params, nil
}

func buildPipelineRequest(params pipelineParams, query backend.DataQuery) (pipelines.ListPipelinesRequest, error) {
	req := pipelines.ListPipelinesRequest{
		Filter: params.Filter,
	}

	return req, nil
}

func (d *Datasource) queryPipelines(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery, qm queryModel) backend.DataResponse {
	params, err := parsePipelineParams(query, qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to parse query params: %v", err))
	}

	w, err := d.getDatabricksClient(ctx, pCtx)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("failed to get databricks client: %v", err))
	}

	request, err := buildPipelineRequest(params, query)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to build request: %v", err))
	}

	pipelinesIter := w.Pipelines.ListPipelines(context.Background(), request)
	pipelines, err := fetchWithLimit(ctx, pipelinesIter, 100)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("failed to list pipelines: %v", err))
	}

	frame := data.NewFrame("pipelines")
	frame.Fields = append(frame.Fields,
		data.NewField("Pipeline Id", nil, []string{}),
		data.NewField("Pipeline Name", nil, []string{}),
	)

	for _, pipeline := range pipelines {
		frame.AppendRow(pipeline.PipelineId, pipeline.Name)
	}

	return backend.DataResponse{
		Frames: []*data.Frame{frame},
	}
}
