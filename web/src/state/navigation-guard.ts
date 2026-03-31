let hasUnsavedChanges = false;

export function setUnsavedChangesGuard(next: boolean) {
  hasUnsavedChanges = next;
}

export function hasUnsavedChangesGuard() {
  return hasUnsavedChanges;
}
