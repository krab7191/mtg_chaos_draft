<script lang="ts">
  import PackRow from './PackRow.svelte';

  let { setName, packs, packOdds, multipliers, onMultChange }: {
    setName: string;
    packs: any[];
    packOdds: Record<string, number>;
    multipliers: Record<string, number>;
    onMultChange: (id: string, value: number) => void;
  } = $props();
</script>

<div class="set-group">
  <div class="set-group__header">
    <span class="set-group__name">{setName}</span>
  </div>
  {#each packs as pack (pack.id)}
    <PackRow
      {pack}
      odds={packOdds[String(pack.id)] ?? 0}
      multiplier={multipliers[String(pack.id)] ?? 1}
      {onMultChange}
    />
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
