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

const triggers = ['issue.created', 'issue.updated', 'comment.added', 'sprint.updated', 'incident.created', 'deployment.created', 'release.created'];

type Condition = { field: string; op: string; value: string };
type Action = { type: string; value: string };

type RuleTemplate = {
  id: string;
  label: string;
  trigger: string;
  conditions: Condition[];
  actions: Action[];
};

const templates: RuleTemplate[] = [
  {
    id: 'done-to-qa',
    label: 'Done → Assign QA',
    trigger: 'issue.updated',
    conditions: [{ field: 'status', op: 'eq', value: 'done' }],
    actions: [{ type: 'assign', value: 'qa-user' }]
  },
  {
    id: 'new-high-priority',
    label: 'New High Priority Alert',
    trigger: 'issue.created',
    conditions: [{ field: 'priority', op: 'eq', value: 'high' }],
    actions: [{ type: 'notify', value: 'on-call' }]
  },
  {
    id: 'comment-escalation',
    label: 'Escalate on blocker comment',
    trigger: 'comment.added',
    conditions: [{ field: 'body', op: 'contains', value: 'blocker' }],
    actions: [{ type: 'set_priority', value: 'high' }]
  }
];

function buildDsl(trigger: string, conditions: Condition[], actions: Action[]) {
  return `IF ${trigger} ${conditions.map((condition) => `${condition.field} ${condition.op} ${condition.value}`).join(' AND ')} THEN ${actions.map((action) => `${action.type} ${action.value}`).join(', ')}`;
}

export function AutomationBuilder({ canEdit }: { canEdit: boolean }) {
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
      dsl: buildDsl(trigger, conditions, actions),
      enabled: true
    }),
    [actions, conditions, name, trigger]
  );

  const previewResult = useMemo(() => {
    const previewEvent = {
      trigger,
      payload: {
        status: 'done',
        priority: 'high',
        body: 'blocker found in production'
      }
    };

    const matches = conditions.every((condition) => {
      const eventValue = String((previewEvent.payload as Record<string, string>)[condition.field] ?? '');
      if (condition.op === 'eq') return eventValue === condition.value;
      if (condition.op === 'ne') return eventValue !== condition.value;
      if (condition.op === 'contains') return eventValue.toLowerCase().includes(condition.value.toLowerCase());
      return false;
    });

    return {
      previewEvent,
      matches,
      actions: matches ? actions : []
    };
  }, [actions, conditions, trigger]);

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

  const applyTemplate = (templateId: string) => {
    if (!templateId) return;
    const template = templates.find((item) => item.id === templateId);
    if (!template) return;
    setName(template.label);
    setTrigger(template.trigger);
    setConditions(template.conditions);
    setActions(template.actions);
    setJsonMode(false);
  };

  const submit = () => {
    if (!canEdit) {
      toast({ title: 'Permission denied', description: 'Only admins can manage automation rules.', variant: 'error' });
      return;
    }

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
          <Button variant="outline" onClick={() => setJsonMode((v) => !v)} disabled={!canEdit}>{jsonMode ? 'Visual mode' : 'JSON mode'}</Button>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="rounded-md border p-3">
          <div className="mb-2 text-xs font-medium text-muted-foreground">Templates</div>
          <Select defaultValue="" onChange={(e) => applyTemplate(e.target.value)} disabled={!canEdit}>
            <option value="">Start from template...</option>
            {templates.map((template) => (
              <option key={template.id} value={template.id}>{template.label}</option>
            ))}
          </Select>
        </div>

        {jsonMode ? (
          <Textarea
            value={jsonPayload}
            onChange={(e) => setJsonPayload(e.target.value)}
            placeholder={JSON.stringify(visualRule, null, 2)}
            className="min-h-[220px]"
            readOnly={!canEdit}
          />
        ) : (
          <>
            <Input placeholder="Rule name" value={name} onChange={(e) => setName(e.target.value)} readOnly={!canEdit} />
            <div className="grid gap-2 md:grid-cols-3">
              <Select value={trigger} onChange={(e) => setTrigger(e.target.value)} disabled={!canEdit}>{triggers.map((t) => <option key={t}>{t}</option>)}</Select>
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
                    readOnly={!canEdit}
                  />
                  <Select
                    value={condition.op}
                    onChange={(e) =>
                      setConditions((prev) => prev.map((item, i) => (i === index ? { ...item, op: e.target.value } : item)))
                    }
                    disabled={!canEdit}
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
                    readOnly={!canEdit}
                  />
                  <Button
                    variant="outline"
                    onClick={() => setConditions((prev) => prev.filter((_, i) => i !== index))}
                    disabled={!canEdit || conditions.length === 1}
                  >
                    Remove
                  </Button>
                </div>
              ))}
              <Button variant="outline" onClick={() => setConditions((prev) => [...prev, { field: '', op: 'eq', value: '' }])} disabled={!canEdit}>Add condition</Button>
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
                    readOnly={!canEdit}
                  />
                  <Input
                    value={action.value}
                    onChange={(e) =>
                      setActions((prev) => prev.map((item, i) => (i === index ? { ...item, value: e.target.value } : item)))
                    }
                    placeholder="Action value"
                    readOnly={!canEdit}
                  />
                  <Button
                    variant="outline"
                    onClick={() => setActions((prev) => prev.filter((_, i) => i !== index))}
                    disabled={!canEdit || actions.length === 1}
                  >
                    Remove
                  </Button>
                </div>
              ))}
              <Button variant="outline" onClick={() => setActions((prev) => [...prev, { type: '', value: '' }])} disabled={!canEdit}>Add action</Button>
            </div>
          </>
        )}

        <div className="rounded-md border bg-muted/20 p-3">
          <div className="text-xs font-medium text-muted-foreground">Preview execution</div>
          <div className="mt-1 text-xs text-muted-foreground">Sample event payload: {JSON.stringify(previewResult.previewEvent.payload)}</div>
          <div className="mt-2 text-sm">
            {previewResult.matches
              ? `Rule would execute ${previewResult.actions.length} action(s): ${previewResult.actions.map((action) => `${action.type}(${action.value})`).join(', ')}`
              : 'Rule conditions do not match the sample event.'}
          </div>
        </div>

        <Button onClick={submit} disabled={createRule.isPending || (!jsonMode && !isVisualValid) || !canEdit}>Save rule</Button>
      </CardContent>
    </Card>
  );
}
