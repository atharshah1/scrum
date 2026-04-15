export type Tokens = {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
};

export type User = {
  id: string;
  org_id: string;
  email: string;
  role: string;
};

export type ApiResponse<T> = {
  success: boolean;
  data: T;
  meta?: {
    page: number;
    limit: number;
    total: number;
  };
  error?: {
    message?: string;
  };
};

export type Project = {
  id: string;
  name: string;
  key?: string;
  description?: string;
};

export type Issue = {
  id: string;
  title: string;
  description?: string;
  status: string;
  priority?: string;
  project_id: string;
  assignee_id?: string;
  labels?: string[];
  sprint_id?: string;
  updated_at?: string;
};

export type IssueComment = {
  id: string;
  issue_id: string;
  author_id?: string;
  body: string;
  created_at: string;
};

export type BoardColumn = {
  id?: string;
  name: string;
  statuses: string[];
  position: number;
  issues: Issue[];
};

export type Board = {
  board_id: string;
  project_id: string;
  columns: BoardColumn[];
};

export type Sprint = {
  id: string;
  board_id: string;
  name: string;
  status: 'planned' | 'active' | 'completed';
  start_at?: string;
  end_at?: string;
};

export type AutomationRule = {
  id?: string;
  name: string;
  trigger: string;
  conditions: Array<Record<string, string>>;
  actions: Array<Record<string, string>>;
  dsl?: string;
  enabled?: boolean;
};

export type WorkflowTransition = {
  id: string;
  from_status: string;
  to_status: string;
  conditions: Record<string, unknown>;
  validators: Record<string, unknown>;
  post_functions: Record<string, unknown>;
};

export type Notification = {
  id: string;
  org_id: string;
  user_id: string;
  type: string;
  title: string;
  message: string;
  entity_id?: string;
  is_read: boolean;
  created_at: string;
};

export type ScrumEvent = {
  id: string;
  org_id: string;
  type: string;
  actor_id: string;
  payload: Record<string, unknown>;
  created_at: string;
};
