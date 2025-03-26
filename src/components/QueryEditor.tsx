import React from 'react';
import { InlineField, Select, Stack } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { JobRunQueryParams, MyDataSourceOptions, MyQuery, PipelineQueryParams } from '../types';
import { JobRunsEditor } from './JobRunsEditor';
import PipelinesEditor from './PipelinesEditor';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const resourceParams = query.resourceParams || {};
  const onResourceTypeChange = (value: SelectableValue<string>) => {
    onChange({ ...query, resourceType:  value.value!});
    onRunQuery();
  };

  const handleQueryChange = (updates: Partial<MyQuery>) => {
    onChange({ ...query, ...updates });
  }

  const resourceEditor = () => {
    switch (query.resourceType) {
      case 'job_runs':
        return (
          <JobRunsEditor
            resourceParams={resourceParams as JobRunQueryParams}
            onChange={handleQueryChange}
            onRunQuery={onRunQuery}
          />
        );

      case 'pipelines':
        return (
          <PipelinesEditor
            resourceParams={resourceParams as PipelineQueryParams}
            onChange={handleQueryChange}
            onRunQuery={onRunQuery}
          />
        );

      default:
        return null;
      }
  }

  return (
    <Stack gap={0}>
      <InlineField label="Resource Type">
        <Select
          options={[
            { label: 'Job Runs', value: 'job_runs' },
            { label: 'Pipelines', value: 'pipelines' },
            { label: 'Pipeline Updates', value: 'pipeline_updates' },
          ]}
          value={query.resourceType}
          onChange={onResourceTypeChange}
        />
      </InlineField>

      {resourceEditor()}
    </Stack>
  );
}
