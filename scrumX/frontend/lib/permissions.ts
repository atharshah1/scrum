const ADMIN_ROLES = new Set(['admin', 'owner']);

export function isAdminRole(role?: string | null) {
  if (!role) return false;
  return ADMIN_ROLES.has(role.toLowerCase());
}

export function canManageAutomation(role?: string | null) {
  return isAdminRole(role);
}

export function canManageOrgSettings(role?: string | null) {
  return isAdminRole(role);
}
