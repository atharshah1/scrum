'use client';

import { DndContext, DragEndEvent, PointerSensor, useSensor, useSensors } from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Board, Issue } from '@/types';

export function BoardView({ boardId, board }: { boardId: string; board: Board }) {
  const queryClient = useQueryClient();
  const sensors = useSensors(useSensor(PointerSensor));

  const moveIssue = useMutation({
    mutationFn: async ({ issueId, status }: { issueId: string; status: string }) =>
      apiRequest(`/issues/${issueId}`, { method: 'PATCH', body: JSON.stringify({ status }) }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.board(boardId) })
  });

  const onDragEnd = (event: DragEndEvent) => {
    const issueId = String(event.active.id);
    const status = event.over?.id ? String(event.over.id) : '';
    if (issueId && status) {
      moveIssue.mutate({ issueId, status });
    }
  };

  return (
    <DndContext sensors={sensors} onDragEnd={onDragEnd}>
      <div className="grid gap-4 md:grid-cols-3">
        {board.columns.map((column) => (
          <Card key={column.name} id={column.statuses[0]}>
            <CardHeader>
              <CardTitle className="flex items-center justify-between">
                <span>{column.name}</span>
                <Badge>{column.issues.length}</Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <SortableContext items={column.issues.map((issue) => issue.id)} strategy={verticalListSortingStrategy}>
                {column.issues.map((issue) => (
                  <IssueCard key={issue.id} issue={issue} />
                ))}
              </SortableContext>
            </CardContent>
          </Card>
        ))}
      </div>
    </DndContext>
  );
}

function IssueCard({ issue }: { issue: Issue }) {
  return (
    <div id={issue.id} className="rounded-md border bg-white p-3 text-sm shadow-sm">
      <div className="font-medium">{issue.title}</div>
      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
        <span>{issue.assignee_id ? `@${issue.assignee_id.slice(0, 6)}` : 'Unassigned'}</span>
        <div className="flex gap-1">
          {(issue.labels ?? []).slice(0, 2).map((label) => (
            <Badge key={label}>{label}</Badge>
          ))}
        </div>
      </div>
    </div>
  );
}
