<script lang="ts">
  interface DraftPick {
    id: number;
    packId: number | null;
    setName: string;
    productType: string;
    marketPrice: number | null;
  }

  interface Draft {
    id: number;
    draftedAt: string;
    approvedAt: string | null;
    approvedBy: number | null;
    picks: DraftPick[];
  }

  import { toast } from '../../lib/toast.svelte';

  let { drafts: initialDrafts, isAdmin }: {
    drafts: Draft[];
    isAdmin: boolean;
  } = $props();

  let drafts = $state<Draft[]>(initialDrafts);
  let approving = $state<Record<number, boolean>>({});

  function removeDraft(draftId: number) {
    toast.confirm('Remove this draft?', async () => {
      const res = await fetch(`/api/drafts/${draftId}`, { method: 'DELETE' });
      if (res.ok) {
        drafts = drafts.filter(d => d.id !== draftId);
      } else {
        toast.show(`Failed to remove draft: ${res.status}`, 'error');
      }
    });
  }

  function formatDate(iso: string): string {
    return new Date(iso).toLocaleString();
  }

  function formatPrice(p: number | null): string {
    return p != null ? `$${p.toFixed(2)}` : '—';
  }

  async function approve(draftId: number) {
    approving[draftId] = true;
    try {
      const res = await fetch(`/api/drafts/${draftId}/approve`, { method: 'POST' });
      if (res.ok) {
        drafts = drafts.map(d =>
          d.id === draftId ? { ...d, approvedAt: new Date().toISOString() } : d
        );
        toast.show('Draft approved', 'success');
      } else {
        const msg = await res.text();
        toast.show(msg.trim() || `Failed to approve: ${res.status}`, 'error');
      }
    } catch {
      toast.show('Failed to approve draft', 'error');
    } finally {
      approving[draftId] = false;
    }
  }
</script>

{#if drafts.length === 0}
  <p class="empty">No drafts recorded yet.</p>
{:else}
  <div class="draft-list">
    {#each drafts as draft (draft.id)}
      <div class="draft-card">
        <div class="draft-card__header">
          <span class="draft-card__date">{formatDate(draft.draftedAt)}</span>
          {#if draft.approvedAt != null}
            <span class="badge badge--approved">Approved</span>
          {/if}
          {#if isAdmin && draft.approvedAt == null}
            <button class="btn-remove" onclick={() => removeDraft(draft.id)}>Remove</button>
            <button
              class="btn-approve"
              disabled={approving[draft.id]}
              onclick={() => approve(draft.id)}
            >{approving[draft.id] ? 'Approving…' : 'Approve'}</button>
          {/if}
        </div>
        <ol class="pick-list">
          {#each draft.picks as pick, i}
            <li class="pick-item">
              <span class="pick-item__num">{i + 1}.</span>
              <span class="pick-item__info">
                <span class="pick-item__name">{pick.setName}</span>
                <span class="pick-item__type">{pick.productType}</span>
              </span>
              <span class="pick-item__price">{formatPrice(pick.marketPrice)}</span>
            </li>
          {/each}
        </ol>
      </div>
    {/each}
  </div>
{/if}

<style>
  .empty {
    color: var(--color-text-muted);
    text-align: center;
    padding: 3rem 0;
  }

  .draft-list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .draft-card {
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .draft-card__header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.65rem 0.9rem;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    flex-wrap: wrap;
  }

  .draft-card__date {
    font-size: 0.85rem;
    font-weight: 500;
    flex: 1;
    min-width: 0;
  }

  .badge--approved {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.15rem 0.55rem;
    border-radius: 999px;
    border: 1px solid #22c55e;
    color: #22c55e;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    white-space: nowrap;
  }

  .btn-remove {
    background: none;
    color: var(--color-text-muted);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.25rem 0.75rem;
    font-size: 0.82rem;
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
  }
  .btn-remove:hover { border-color: var(--color-danger); color: var(--color-danger); }

  .btn-approve {
    background: var(--color-accent);
    color: white;
    border: none;
    border-radius: var(--radius);
    padding: 0.25rem 0.75rem;
    font-size: 0.82rem;
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
  }
  .btn-approve:hover { opacity: 0.85; }
  .btn-approve:disabled { opacity: 0.5; cursor: default; }

  .pick-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .pick-item {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
    padding: 0.4rem 0.9rem;
    border-bottom: 1px solid var(--color-border);
    font-size: 0.85rem;
  }
  .pick-item:last-child { border-bottom: none; }

  .pick-item__num {
    color: var(--color-text-muted);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
    min-width: 1.2rem;
  }

  .pick-item__info {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 0.35rem;
    align-items: baseline;
    overflow: hidden;
  }

  .pick-item__name {
    font-weight: 500;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .pick-item__type {
    color: var(--color-text-muted);
    font-size: 0.78rem;
    white-space: nowrap;
  }

  .pick-item__price {
    color: var(--color-text-muted);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
</style>
