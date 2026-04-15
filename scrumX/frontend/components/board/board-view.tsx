'use client';

import { useMemo, useState } from 'react';
import { CSS } from '@dnd-kit/utilities';
import { DndContext, DragEndEvent, PointerSensor, useDraggable, useDroppable, useSensor, useSensors } from '@dnd-kit/core';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { toast } from '@/components/ui/toast';
import { apiRequest } from '@/lib/api';
import { formatAssignee } from '@/lib/format';
import { qk } from '@/lib/query-keys';
import type { Board, Issue, WorkflowTransition } from '@/types';

function canTransition(status: string, nextStatus: string, transitions: WorkflowTransition[]) {
  return transitions.some((transition) => transition.from_status === status && transition.to_status === nextStatus);
}

function moveIssueInBoard(board: Board, issueId: string, targetStatus: string): Board {
  let movedIssue: Issue | null = null;
  const columnsWithoutIssue = board.columns.map((column) => ({
    ...column,
    issues: column.issues.filter((issue) => {
      if (issue.id === issueId) {
        movedIssue = { ...issue, status: targetStatus };
        return false;
      }
      return true;
    })
  }));

  if (!movedIssue) return board;

  return {
    ...board,
    columns: columnsWithoutIssue.map((column) => {
      const accepts = column.statuses.includes(targetStatus);
      if (!accepts) return column;
      return { ...column, issues: [movedIssue as Issue, ...column.issues] };
    })
  };
}

export function BoardView({ boardId, board, transitions }: { boardId: string; board: Board; transitions: WorkflowTransition[] }) {
  const queryClient = useQueryClient();
  const sensors = useSensors(useSensor(PointerSensor));
  const issuesById = useMemo(
    () =>
      board.columns.reduce<Record<string, Issue>>((acc, column) => {
        column.issues.forEach((issue) => {
          acc[issue.id] = issue;
        });
        return acc;
      }, {}),
    [board.columns]
  );

  const moveIssue = useMutation({
    mutationFn: async ({ issueId, status }: { issueId: string; status: string }) =>
      apiRequest(`/issues/${issueId}`, { method: 'PATCH', body: JSON.stringify({ status }) }),
    onMutate: async ({ issueId, status }) => {
      await queryClient.cancelQueries({ queryKey: qk.board(boardId) });
      const previous = queryClient.getQueryData<Board>(qk.board(boardId));
      if (previous) {
        queryClient.setQueryData<Board>(qk.board(boardId), moveIssueInBoard(previous, issueId, status));
      }
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(qk.board(boardId), context.previous);
      }
      toast({ title: 'Move failed', description: 'Issue transition was rejected and has been reverted.', variant: 'error' });
    },
    onSuccess: (_data, vars) => {
      queryClient.setQueryData<Issue | undefined>(qk.issue(vars.issueId), (current) =>
        current ? { ...current, status: vars.status } : current
      );
    },
    onSettled: (_data, _error, vars) => {
      queryClient.invalidateQueries({ queryKey: qk.board(boardId), exact: true });
      queryClient.invalidateQueries({ queryKey: qk.issue(vars.issueId), exact: true });
    }
  });

  const onDragEnd = (event: DragEndEvent) => {
    const issueId = String(event.active.id);
    const targetStatus = event.over?.id ? String(event.over.id) : '';
    if (!issueId || !targetStatus) return;

    const activeIssue = issuesById[issueId];
    if (!activeIssue || activeIssue.status === targetStatus) return;

    if (!canTransition(activeIssue.status, targetStatus, transitions)) {
      toast({
        title: 'Transition not allowed',
        description: `${activeIssue.status} → ${targetStatus} is not allowed by workflow rules.`,
        variant: 'error'
      });
      return;
    }

    moveIssue.mutate({ issueId, status: targetStatus });
  };

  return (
    <DndContext sensors={sensors} onDragEnd={onDragEnd}>
      <div className="grid gap-4 md:grid-cols-3">
        {board.columns.map((column) => (
          <BoardColumnCard key={column.name} column={column} transitions={transitions} />
        ))}
      </div>
    </DndContext>
  );
}

function BoardColumnCard({ column, transitions }: { column: Board['columns'][number]; transitions: WorkflowTransition[] }) {
  const targetStatus = column.statuses[0] ?? column.name.toLowerCase();
  const { setNodeRef, isOver } = useDroppable({ id: targetStatus });
  const [visibleCount, setVisibleCount] = useState(80);
  const visibleIssues = column.issues.slice(0, visibleCount);
  const hasMore = column.issues.length > visibleCount;

  return (
    <div ref={setNodeRef}>
      <Card className={isOver ? 'ring-2 ring-blue-200' : ''}>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>{column.name}</span>
            <Badge>{column.issues.length}</Badge>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {visibleIssues.map((issue) => (
            <IssueCard key={issue.id} issue={issue} transitions={transitions} />
          ))}
          {hasMore ? (
            <button
              type="button"
              className="w-full rounded-md border border-dashed p-2 text-xs text-muted-foreground hover:bg-accent"
              onClick={() => setVisibleCount((count) => count + 80)}
            >
              Show 80 more issues
            </button>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function IssueCard({ issue, transitions }: { issue: Issue; transitions: WorkflowTransition[] }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: issue.id });

  const style = {
    transform: CSS.Translate.toString(transform),
    opacity: isDragging ? 0.65 : 1
  };

  const allowedStatuses = transitions
    .filter((transition) => transition.from_status === issue.status)
    .map((transition) => transition.to_status);

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="cursor-grab rounded-md border bg-white p-3 text-sm shadow-sm active:cursor-grabbing"
      {...listeners}
      {...attributes}
    >
      <div className="font-medium">{issue.title}</div>
      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
        <span>{formatAssignee(issue.assignee_id)}</span>
        <div className="flex gap-1">
          {(issue.labels ?? []).slice(0, 2).map((label) => (
            <Badge key={label}>{label}</Badge>
          ))}
        </div>
      </div>
      <div className="mt-2 flex flex-wrap gap-1">
        {allowedStatuses.map((status) => (
          <Badge key={status} className="bg-slate-100 text-slate-700" aria-label={`Transition to ${status}`}>
            <span aria-hidden>→ {status}</span>
          </Badge>
        ))}
      </div>
    </div>
  );
}
