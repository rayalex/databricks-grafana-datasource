import React from 'react';
import { InlineField, Select, Stack } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onResourceTypeChange = (value: SelectableValue<string>) => {
    onChange({ ...query, resourceType:  value.value!});
    onRunQuery();
  };

  return (
    <Stack gap={0}>
      <InlineField label="Resource Type">
        <Select
          options={[
            { label: 'Job Runs', value: 'job_runs' },
          ]}
          value={query.resourceType}
          onChange={onResourceTypeChange}
        />
      </InlineField>
    </Stack>
  );
}
