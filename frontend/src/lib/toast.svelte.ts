type ToastType = 'info' | 'error' | 'success';

class ToastStore {
  visible = $state(false);
  msg     = $state('');
  type    = $state<ToastType>('info');
  mode    = $state<'info' | 'confirm'>('info');

  // Not reactive — only read on button click
  onConfirm: (() => void) | undefined;
  onCancel:  (() => void) | undefined;

  #timer: ReturnType<typeof setTimeout> | null = null;

  show(msg: string, type: ToastType = 'info') {
    this.#clear();
    this.msg  = msg;
    this.type = type;
    this.mode = 'info';
    this.visible = true;
    this.#timer = setTimeout(() => this.dismiss(), 4000);
  }

  confirm(msg: string, onConfirm: () => void, onCancel?: () => void) {
    this.#clear();
    this.msg       = msg;
    this.type      = 'info';
    this.mode      = 'confirm';
    this.onConfirm = onConfirm;
    this.onCancel  = onCancel;
    this.visible   = true;
  }

  dismiss() {
    this.#clear();
    this.visible = false;
  }

  #clear() {
    if (this.#timer) { clearTimeout(this.#timer); this.#timer = null; }
  }
}

export const toast = new ToastStore();
