package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func (d *Datasource) queryPipelines(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery, qm queryModel) backend.DataResponse {
	return backend.DataResponse{}
}
