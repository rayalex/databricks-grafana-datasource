import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export interface MyQuery extends DataQuery {
  resourceType: string;
  resourceParams: ResourceParams;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  resourceType: "job_runs",
};

export type ResourceParams = JobRunQueryParams;

export interface JobRunQueryParams { }

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  workspace?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  clientId?: string;
  clientSecret?: string;
}
