// Thin adapter over svelte-sonner so call sites keep a small, stable API.
import { toast as sonner } from 'svelte-sonner';

export type ToastType = 'info' | 'error' | 'success';

/** Errors linger longer: they're the ones people need time to read. */
export const DURATIONS: Record<ToastType, number> = { success: 4000, info: 5000, error: 8000 };

export const toast = {
  show(msg: string, type: ToastType = 'info'): string | number {
    // Keying on the message means a repeat (e.g. a search failing on every
    // keystroke) refreshes the existing toast instead of stacking duplicates.
    const opts = { id: `${type}:${msg}`, duration: DURATIONS[type] };
    if (type === 'error') return sonner.error(msg, opts);
    if (type === 'success') return sonner.success(msg, opts);
    return sonner.info(msg, opts);
  },

  error(msg: string)   { return this.show(msg, 'error'); },
  success(msg: string) { return this.show(msg, 'success'); },

  /** A question that waits for an answer: stays until a button is pressed. */
  confirm(msg: string, onConfirm: () => void, onCancel?: () => void, confirmLabel = 'Remove'): string | number {
    return sonner(msg, {
      id: 'confirm', // one pending question at a time
      duration: Number.POSITIVE_INFINITY,
      action: { label: confirmLabel, onClick: () => onConfirm() },
      cancel: { label: 'Cancel', onClick: () => onCancel?.() },
    });
  },

  dismiss(id?: string | number) { sonner.dismiss(id); },
};
