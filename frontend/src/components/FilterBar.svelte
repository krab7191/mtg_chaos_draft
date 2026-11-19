<script lang="ts">
  // One filter bar for every prices page; settings are shared and remembered.
  import { LANGUAGES } from '../lib/deals';
  import { filters, ALL_LANGUAGES, setLanguage, setMaxPrice, clearFilters, languageShown } from '../lib/filters.svelte';

  const active = $derived(filters.languages != null || filters.maxPrice != null);
</script>

<div class="bar" role="group" aria-label="Filters for all price pages">
  <fieldset class="langs">
    <legend>Languages</legend>
    {#each ALL_LANGUAGES as lang (lang)}
      <label class:off={!languageShown(lang)}>
        <input type="checkbox" checked={languageShown(lang)} onchange={e => setLanguage(lang, e.currentTarget.checked)} />
        {LANGUAGES[lang]}
      </label>
    {/each}
  </fieldset>
  <label class="max">
    Max price $
    <input type="number" min="0" step="1" inputmode="decimal" placeholder="any" value={filters.maxPrice ?? ''}
      onchange={e => { setMaxPrice(parseFloat(e.currentTarget.value)); e.currentTarget.value = filters.maxPrice == null ? '' : String(filters.maxPrice); }} />
  </label>
  {#if active}<button type="button" class="clear" onclick={clearFilters}>Clear</button>{/if}
</div>

<style>
  .bar {
    display: flex; flex-wrap: wrap; gap: 0.4rem 1.2rem; align-items: center;
    padding: 0.5rem 0.75rem; margin-bottom: 1.25rem;
    border: 1px solid var(--color-border); border-radius: var(--radius); background: var(--color-surface);
    font-size: 0.8rem; color: var(--color-text-muted);
  }
  .langs { border: none; padding: 0; margin: 0; display: flex; flex-wrap: wrap; gap: 0.2rem 0.7rem; align-items: center; }
  .langs legend { float: left; margin-right: 0.5rem; color: var(--color-text); }
  label { display: inline-flex; gap: 0.25rem; align-items: center; cursor: pointer; color: var(--color-text); }
  label.off { color: var(--color-text-muted); text-decoration: line-through; }
  .max input { width: 4.5rem; font: inherit; }
  .clear { background: none; border: none; color: var(--color-accent); font: inherit; cursor: pointer; padding: 0; }
</style>
