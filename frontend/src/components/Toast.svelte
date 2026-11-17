<script lang="ts">
  import { Toaster } from 'svelte-sonner';
</script>

<!--
  svelte-sonner handles stacking, swipe-to-dismiss, pause-on-hover and
  screen-reader announcements. Toasts sit above a page's fixed bottom bar
  when the page sets --toast-offset (see PackSelector).
-->
<Toaster
  theme="dark"
  position="bottom-right"
  closeButton
  richColors
  visibleToasts={3}
  offset={{ bottom: 'calc(1rem + max(var(--toast-offset, 0px), env(safe-area-inset-bottom)))' }}
  mobileOffset={{ bottom: 'calc(0.75rem + max(var(--toast-offset, 0px), env(safe-area-inset-bottom)))' }}
/>
