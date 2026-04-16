// Keep this short enough for dense cards while still being recognizable in lists/details.
const ASSIGNEE_ID_DISPLAY_LENGTH = 8;

export function formatAssignee(assigneeId?: string) {
  if (!assigneeId) return 'Unassigned';
  return `@${assigneeId.slice(0, ASSIGNEE_ID_DISPLAY_LENGTH)}`;
}
