'use client';

import { useParams, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { BoardView } from '@/components/board/board-view';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { apiRequest } from '@/lib/api';
import { qk } from '@/lib/query-keys';
import type { Board } from '@/types';

export default function BoardPage() {
  const params = useParams<{ projectId: string }>();
  const searchParams = useSearchParams();
  const boardId = searchParams.get('boardId') ?? params.projectId;

  const boardQuery = useQuery({
    queryKey: qk.board(boardId),
    queryFn: () => apiRequest<Board>(`/boards/${boardId}`)
  });

  if (!boardQuery.data) {
    return (
      <Card>
        <CardHeader><CardTitle>Board</CardTitle></CardHeader>
        <CardContent>Loading board...</CardContent>
      </Card>
    );
  }

  return <BoardView boardId={boardId} board={boardQuery.data} />;
}
