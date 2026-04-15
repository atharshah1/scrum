'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { toast } from '@/components/ui/toast';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { AutomationRule } from '@/types';

const triggers = ['issue.created', 'issue.updated', 'comment.added', 'sprint.updated'];

type Condition = { field: string; op: string; value: string };

type Action = { type: string; value: string };

export function AutomationBuilder() {
  const [name, setName] = useState('');
  const [trigger, setTrigger] = useState(triggers[0]);
  const [conditions, setConditions] = useState<Condition[]>([{ field: 'status', op: 'eq', value: 'done' }]);
  const [actions, setActions] = useState<Action[]>([{ type: 'assign', value: 'qa-user' }]);
  const [jsonMode, setJsonMode] = useState(false);
  const [jsonPayload, setJsonPayload] = useState('');

  const queryClient = useQueryClient();

  const visualRule = useMemo<AutomationRule>(
    () => ({
      name,
      trigger,
      conditions: conditions.map((condition) => ({ field: condition.field, op: condition.op, value: condition.value })),
      actions: actions.map((action) => ({ type: action.type, value: action.value })),
      dsl: `IF ${trigger} ${conditions.map((condition) => `${condition.field} ${condition.op} ${condition.value}`).join(' AND ')} THEN ${actions.map((action) => `${action.type} ${action.value}`).join(', ')}`,
      enabled: true
    }),
    [actions, conditions, name, trigger]
  );

  const createRule = useMutation({
    mutationFn: async (rule: AutomationRule) => apiRequest('/automation/rules', { method: 'POST', body: JSON.stringify(rule) }),
    onSuccess: () => {
      setName('');
      setConditions([{ field: 'status', op: 'eq', value: 'done' }]);
      setActions([{ type: 'assign', value: 'qa-user' }]);
      queryClient.invalidateQueries({ queryKey: qk.automationRules, exact: true });
      toast({ title: 'Automation rule saved', variant: 'success' });
    }
  });

  const isVisualValid =
    name.trim().length > 0 &&
    conditions.length > 0 &&
    actions.length > 0 &&
    conditions.every((c) => c.field.trim() && c.op.trim() && c.value.trim()) &&
    actions.every((a) => a.type.trim() && a.value.trim());

  const submit = () => {
    if (jsonMode) {
      try {
        const parsed = JSON.parse(jsonPayload) as AutomationRule;
        if (!parsed.name || !parsed.trigger) {
          toast({ title: 'Invalid JSON rule', description: 'Name and trigger are required.', variant: 'error' });
          return;
        }
        createRule.mutate(parsed);
      } catch {
        toast({ title: 'Invalid JSON', description: 'Please provide valid JSON.', variant: 'error' });
      }
      return;
    }

    if (!isVisualValid) {
      toast({ title: 'Invalid rule', description: 'Fill all condition and action fields.', variant: 'error' });
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
            </div>

            <div className="space-y-2 rounded-md border p-3">
              <div className="text-xs font-medium text-muted-foreground">Conditions (AND chain)</div>
              {conditions.map((condition, index) => (
                <div key={`condition-${index}`} className="grid gap-2 md:grid-cols-[1fr_120px_1fr_auto]">
                  <Input
                    value={condition.field}
                    onChange={(e) =>
                      setConditions((prev) => prev.map((item, i) => (i === index ? { ...item, field: e.target.value } : item)))
                    }
                    placeholder="Field"
                  />
                  <Select
                    value={condition.op}
                    onChange={(e) =>
                      setConditions((prev) => prev.map((item, i) => (i === index ? { ...item, op: e.target.value } : item)))
                    }
                  >
                    <option value="eq">equals</option>
                    <option value="ne">not equals</option>
                    <option value="contains">contains</option>
                  </Select>
                  <Input
                    value={condition.value}
                    onChange={(e) =>
                      setConditions((prev) => prev.map((item, i) => (i === index ? { ...item, value: e.target.value } : item)))
                    }
                    placeholder="Value"
                  />
                  <Button
                    variant="outline"
                    onClick={() => setConditions((prev) => prev.filter((_, i) => i !== index))}
                    disabled={conditions.length === 1}
                  >
                    Remove
                  </Button>
                </div>
              ))}
              <Button variant="outline" onClick={() => setConditions((prev) => [...prev, { field: '', op: 'eq', value: '' }])}>Add condition</Button>
            </div>

            <div className="space-y-2 rounded-md border p-3">
              <div className="text-xs font-medium text-muted-foreground">Actions</div>
              {actions.map((action, index) => (
                <div key={`action-${index}`} className="grid gap-2 md:grid-cols-[1fr_1fr_auto]">
                  <Input
                    value={action.type}
                    onChange={(e) =>
                      setActions((prev) => prev.map((item, i) => (i === index ? { ...item, type: e.target.value } : item)))
                    }
                    placeholder="Action type"
                  />
                  <Input
                    value={action.value}
                    onChange={(e) =>
                      setActions((prev) => prev.map((item, i) => (i === index ? { ...item, value: e.target.value } : item)))
                    }
                    placeholder="Action value"
                  />
                  <Button
                    variant="outline"
                    onClick={() => setActions((prev) => prev.filter((_, i) => i !== index))}
                    disabled={actions.length === 1}
                  >
                    Remove
                  </Button>
                </div>
              ))}
              <Button variant="outline" onClick={() => setActions((prev) => [...prev, { type: '', value: '' }])}>Add action</Button>
            </div>
          </>
        )}
        <Button onClick={submit} disabled={createRule.isPending || (!jsonMode && !isVisualValid)}>Save rule</Button>
      </CardContent>
    </Card>
  );
}
