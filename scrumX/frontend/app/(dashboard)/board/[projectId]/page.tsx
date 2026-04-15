'use client';

import { useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { BoardView } from '@/components/board/board-view';
import { Skeleton } from '@/components/ui/skeleton';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Board, StuckInsightResponse, WorkflowTransition } from '@/types';

const STUCK_THRESHOLD_DAYS = 2;

const defaultTransitions: WorkflowTransition[] = [
  { id: 'todo-in-progress', from_status: 'todo', to_status: 'in_progress', conditions: {}, validators: {}, post_functions: {} },
  { id: 'todo-done', from_status: 'todo', to_status: 'done', conditions: {}, validators: {}, post_functions: {} },
  { id: 'in-progress-todo', from_status: 'in_progress', to_status: 'todo', conditions: {}, validators: {}, post_functions: {} },
  { id: 'in-progress-done', from_status: 'in_progress', to_status: 'done', conditions: {}, validators: {}, post_functions: {} },
  { id: 'done-todo', from_status: 'done', to_status: 'todo', conditions: {}, validators: {}, post_functions: {} }
];

export default function BoardPage() {
  const params = useParams<{ projectId: string }>();
  const searchParams = useSearchParams();
  const boardId = searchParams.get('boardId') ?? params.projectId;
  const [showOnlyStuck, setShowOnlyStuck] = useState(false);

  const boardQuery = useQuery({
    queryKey: qk.board(boardId),
    queryFn: () => apiRequest<Board>(`/boards/${boardId}`)
  });

  const transitionsQuery = useQuery({
    queryKey: qk.workflowTransitions(boardQuery.data?.project_id ?? ''),
    enabled: !!boardQuery.data?.project_id,
    queryFn: () => apiRequest<WorkflowTransition[]>(`/workflows/${boardQuery.data?.project_id}/transitions`)
  });

  const transitions = useMemo(() => {
    if (transitionsQuery.data && transitionsQuery.data.length) return transitionsQuery.data;
    return defaultTransitions;
  }, [transitionsQuery.data]);

  const stuckQuery = useQuery({
    queryKey: qk.insightsStuck(boardId),
    queryFn: () =>
      apiRequest<StuckInsightResponse>(
        `/insights/stuck?project_id=${boardQuery.data?.project_id ?? ''}&threshold_days=${STUCK_THRESHOLD_DAYS}`
      ),
    enabled: !!boardQuery.data?.project_id
  });

  const filteredBoard = useMemo(() => {
    if (!boardQuery.data || !showOnlyStuck) return boardQuery.data;
    const stuckIDs = new Set((stuckQuery.data?.items ?? []).map((item) => item.id));
    return {
      ...boardQuery.data,
      columns: boardQuery.data.columns.map((column) => ({
        ...column,
        issues: column.issues.filter((issue) => stuckIDs.has(issue.id))
      }))
    };
  }, [boardQuery.data, showOnlyStuck, stuckQuery.data]);

  if (boardQuery.isPending) {
    return (
      <Card>
        <CardHeader><CardTitle>Board</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-6 w-32" />
          <div className="grid gap-4 md:grid-cols-3">
            <Skeleton className="h-40" />
            <Skeleton className="h-40" />
            <Skeleton className="h-40" />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!boardQuery.data) {
    return (
      <Card>
        <CardHeader><CardTitle>Board</CardTitle></CardHeader>
        <CardContent>Unable to load board.</CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {(stuckQuery.data?.items.length ?? 0) > 0 ? (
        <Card>
          <CardContent className="flex items-center justify-between gap-3 p-3 text-sm">
            <span>⚠ {stuckQuery.data?.items.length} stuck tasks older than {stuckQuery.data?.threshold_days ?? STUCK_THRESHOLD_DAYS} days.</span>
            <Button size="sm" variant="outline" onClick={() => setShowOnlyStuck((value) => !value)}>
              {showOnlyStuck ? 'Show all' : 'Filter stuck'}
            </Button>
          </CardContent>
        </Card>
      ) : null}
      <BoardView boardId={boardId} board={filteredBoard ?? boardQuery.data} transitions={transitions} />
    </div>
  );
}
