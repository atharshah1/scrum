'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { AutomationRule } from '@/types';

const triggers = ['issue.created', 'issue.updated', 'comment.added', 'sprint.updated'];

export function AutomationBuilder() {
  const [name, setName] = useState('');
  const [trigger, setTrigger] = useState(triggers[0]);
  const [conditionField, setConditionField] = useState('status');
  const [conditionValue, setConditionValue] = useState('done');
  const [actionType, setActionType] = useState('assign');
  const [actionValue, setActionValue] = useState('qa-user');
  const [jsonMode, setJsonMode] = useState(false);
  const [jsonPayload, setJsonPayload] = useState('');

  const queryClient = useQueryClient();

  const visualRule = useMemo<AutomationRule>(
    () => ({
      name,
      trigger,
      conditions: [{ field: conditionField, op: 'eq', value: conditionValue }],
      actions: [{ type: actionType, value: actionValue }],
      dsl: `IF ${trigger} AND ${conditionField} = ${conditionValue} THEN ${actionType} ${actionValue}`,
      enabled: true
    }),
    [actionType, actionValue, conditionField, conditionValue, name, trigger]
  );

  const createRule = useMutation({
    mutationFn: async (rule: AutomationRule) => apiRequest('/automation/rules', { method: 'POST', body: JSON.stringify(rule) }),
    onSuccess: () => {
      setName('');
      queryClient.invalidateQueries({ queryKey: qk.automationRules });
    }
  });

  const submit = () => {
    if (jsonMode) {
      createRule.mutate(JSON.parse(jsonPayload) as AutomationRule);
      return;
    }
    createRule.mutate(visualRule);
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span>Automation builder</span>
          <Button variant="outline" onClick={() => setJsonMode((v) => !v)}>{jsonMode ? 'Visual mode' : 'JSON mode'}</Button>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {jsonMode ? (
          <Textarea
            value={jsonPayload}
            onChange={(e) => setJsonPayload(e.target.value)}
            placeholder={JSON.stringify(visualRule, null, 2)}
            className="min-h-[220px]"
          />
        ) : (
          <>
            <Input placeholder="Rule name" value={name} onChange={(e) => setName(e.target.value)} />
            <div className="grid gap-2 md:grid-cols-3">
              <Select value={trigger} onChange={(e) => setTrigger(e.target.value)}>{triggers.map((t) => <option key={t}>{t}</option>)}</Select>
              <Input value={conditionField} onChange={(e) => setConditionField(e.target.value)} placeholder="Condition field" />
              <Input value={conditionValue} onChange={(e) => setConditionValue(e.target.value)} placeholder="Condition value" />
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              <Input value={actionType} onChange={(e) => setActionType(e.target.value)} placeholder="Action type" />
              <Input value={actionValue} onChange={(e) => setActionValue(e.target.value)} placeholder="Action value" />
            </div>
          </>
        )}
        <Button onClick={submit} disabled={createRule.isPending || (!jsonMode && !name.trim())}>Save rule</Button>
      </CardContent>
    </Card>
  );
}
