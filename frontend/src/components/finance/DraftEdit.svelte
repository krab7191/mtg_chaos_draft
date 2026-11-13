<script lang="ts">
  import { untrack } from 'svelte';
  import { send } from '../../lib/send';
  import type { Disposal } from '../../lib/portfolio';

  let { draftId, collected, onsaved, oncancel }: {
    draftId: number;
    collected: number;
    onsaved: (records: Disposal[]) => void;
    oncancel: () => void;
  } = $props();

  let amount = $state<number | null>(untrack(() => Math.round(collected * 100) / 100));
  let saving = $state(false);
  const ok = $derived(amount != null && amount >= 0 && !saving);

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!ok) return;
    saving = true;
    const records = await send<Disposal[]>(`/api/drafts/${draftId}/collected`, 'PUT', { amount }, 'Failed to save');
    saving = false;
    if (records && records !== true) onsaved(records);
  }
</script>

<form class="form form--inline" onsubmit={save}>
  <label class="field">
    <span>Collected from players</span>
    <input type="number" min="0" step="0.01" inputmode="decimal" placeholder="$ total" bind:value={amount} required />
  </label>
  <div class="form__actions">
    <button class="btn-save" type="submit" disabled={!ok}>{saving ? 'Saving…' : 'Save'}</button>
    <button class="btn-row" type="button" onclick={oncancel}>Cancel</button>
  </div>
  <p class="form__hint">The total you actually received. Leave cards you kept out of it: those count as a cost.</p>
</form>
