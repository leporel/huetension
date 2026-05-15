/**
 * Global undo / redo keyboard shortcuts for the workspace palette.
 *
 *   Ctrl/Cmd + Z         → undo
 *   Ctrl/Cmd + Shift + Z → redo
 *   Ctrl/Cmd + Y         → redo
 *
 * Skipped while a form field is focused so the browser's native text
 * undo keeps working inside hex inputs / selects. Mounted once from
 * App.vue — every card writes through the one workspace history, so a
 * single global listener covers wheel, picker, image pins, and the
 * library "load into workspace" action alike.
 */

import { onBeforeUnmount, onMounted } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';

function isEditable(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  return (
    tag === 'INPUT' ||
    tag === 'TEXTAREA' ||
    tag === 'SELECT' ||
    el.isContentEditable
  );
}

export function useWorkspaceShortcuts(): void {
  const workspace = useWorkspaceStore();

  function onKeydown(e: KeyboardEvent): void {
    if (!(e.ctrlKey || e.metaKey)) return;
    if (isEditable(e.target)) return;

    const key = e.key.toLowerCase();
    const wantsRedo = key === 'y' || (key === 'z' && e.shiftKey);
    const wantsUndo = key === 'z' && !e.shiftKey;

    if (wantsUndo && workspace.canUndo) {
      e.preventDefault();
      workspace.undo();
    } else if (wantsRedo && workspace.canRedo) {
      e.preventDefault();
      workspace.redo();
    }
  }

  onMounted(() => window.addEventListener('keydown', onKeydown));
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown));
}
