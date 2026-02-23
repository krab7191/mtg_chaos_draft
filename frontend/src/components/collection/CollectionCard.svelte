<script lang="ts">
  import CollectionRow from './CollectionRow.svelte';

  let { setName, packs, onQtyChange, onDelete }: {
    setName: string;
    packs: any[];
    onQtyChange: (id: number, delta: number) => void;
    onDelete: (id: number, label: string) => void;
  } = $props();
</script>

<div class="set-group">
  <div class="set-group__header">
    <span class="set-group__name">{setName}</span>
  </div>
  {#each packs as pack (pack.id)}
    <CollectionRow {pack} {onQtyChange} {onDelete} />
  {/each}
</div>

<style>
  .set-group {
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .set-group__header {
    padding: 0.45rem 0.75rem;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
  }

  .set-group__name {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--color-text);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
</style>
