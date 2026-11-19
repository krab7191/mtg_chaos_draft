<script lang="ts">
  import { onMount } from 'svelte';
  import { getJSON } from '../../lib/api';
  import { money } from '../../lib/trends';
  import { languageLabel } from '../../lib/deals';
  import { filters } from '../../lib/filters.svelte';
  import { rank, topPicks, reasons, type BestDealsResponse, type ShippingSettings } from '../../lib/bestdeals';

  let data = $state<BestDealsResponse | null>(null);
  let loading = $state(true);

  // Language and max price come from the shared filter bar above the page.

  const SHIP_KEY = 'bestDealsShipping';
  let ship = $state<ShippingSettings>({ perOrder: {}, packsPerOrder: 3 });
  $effect(() => {
    if (!data) return;
    try { localStorage.setItem(SHIP_KEY, JSON.stringify(ship)); } catch { /* private mode: not remembered */ }
  });

  onMount(async () => {
    const res = await getJSON<BestDealsResponse>('/api/best-deals', "Couldn't load the best deals");
    if (res) {
      let saved: Partial<ShippingSettings> = {};
      try { saved = JSON.parse(localStorage.getItem(SHIP_KEY) ?? '{}'); } catch { /* none */ }
      ship = { perOrder: { ...res.shipping, ...(saved.perOrder ?? {}) }, packsPerOrder: saved.packsPerOrder ?? 3 };
      data = res;
    }
    loading = false;
  });

  const ranked = $derived(data ? rank(data.candidates, filters, ship) : []);
  const picks = $derived(topPicks(ranked, 3));
  const runnersUp = $derived(topPicks(ranked, 10).slice(3));
</script>

<section class="best">
  {#if loading}
    <p class="note">Loading…</p>
  {:else if !data}
    <p class="note">Best deals aren't available right now.</p>
  {:else if picks.length === 0}
    <p class="note">Nothing to rank yet: pack contents values and store listings load after the server's daily run.</p>
  {:else}
    <ol class="picks">
      {#each picks as p, i (p.mtgstocksId + p.store + p.url)}
        <li class="pick">
          <div class="pick__rank">{i + 1}</div>
          <div class="pick__body">
            <h2 class="pick__name">
              {p.setName} {p.productType}
              {#if languageLabel(p.language)}<span class="lang">{languageLabel(p.language)}</span>{/if}
            </h2>
            <p class="pick__buy">
              <strong>{money(p.price)}</strong> at
              {#if p.url}<a href={p.url} target="_blank" rel="noopener noreferrer">{p.store}</a>{:else}{p.store}{/if}
              <span class="muted">(about {money(p.delivered)} with shipping)</span>
            </p>
            <ul class="pick__why">
              {#each reasons(p) as why (why)}<li>{why}</li>{/each}
            </ul>
          </div>
        </li>
      {/each}
    </ol>

    {#if runnersUp.length > 0}
      <details class="more">
        <summary>Runners-up</summary>
        <ol start="4" class="runners">
          {#each runnersUp as p (p.mtgstocksId + p.store + p.url)}
            <li>
              {p.setName} {p.productType}{#if languageLabel(p.language)} ({languageLabel(p.language)}){/if} —
              {money(p.price)} at {p.store}, ${p.perDollar.toFixed(2)} of cards per $1
            </li>
          {/each}
        </ol>
      </details>
    {/if}

    <details class="settings">
      <summary>How these are picked · shipping</summary>
      <p class="note">
        Every in-stock pack (Card Kingdom, CardTrader, and TCGplayer at its market price) is scored on
        <strong>contents ÷ (price + shipping per pack)</strong>, then adjusted by its price signal: how much packs with that
        signal moved over the following 3 months in years of history (climbing ones rose ~8–12%; new sets fell ~5–6%).
        The top 3 are from different sets. Unreleased sets, products without real sales and listings far above market are
        left out. Contents count every card at market price, including bulk commons that are hard to sell.
      </p>
      <p class="note">Language and max price come from the filter bar at the top of the page.</p>
      <div class="ship">
        <span>Shipping per order:</span>
        {#each Object.keys(ship.perOrder) as store (store)}
          <label>{store} $<input type="number" min="0" step="0.5" value={ship.perOrder[store]}
            onchange={e => { const v = parseFloat(e.currentTarget.value); ship.perOrder[store] = Number.isFinite(v) && v >= 0 ? v : 0; e.currentTarget.value = String(ship.perOrder[store]); }} /></label>
        {/each}
        <label>Packs per order <input type="number" min="1" step="1" value={ship.packsPerOrder}
          onchange={e => { const v = parseInt(e.currentTarget.value, 10); ship.packsPerOrder = Number.isFinite(v) && v >= 1 ? v : 1; e.currentTarget.value = String(ship.packsPerOrder); }} /></label>
      </div>
      <p class="note">Shipping guesses: CardTrader Zero's US rate starts at $3.37; Card Kingdom's free shipping covers singles, not packs. Adjust to what you actually pay.</p>
    </details>
  {/if}
</section>

<style>
  .note { font-size: 0.85rem; color: var(--color-text-muted); }
  .muted { color: var(--color-text-muted); font-size: 0.85rem; }
  .picks { list-style: none; padding: 0; margin: 0 0 1rem; display: grid; gap: 0.8rem; }
  .pick { display: flex; gap: 0.9rem; padding: 0.8rem 1rem; border: 1px solid var(--color-border); border-radius: var(--radius); background: var(--color-surface); }
  .pick__rank { font-size: 1.6rem; font-weight: 700; color: var(--tone-great); min-width: 1.6rem; }
  .pick__name { font-size: 1rem; margin: 0 0 0.25rem; }
  .pick__buy { margin: 0 0 0.4rem; font-size: 0.9rem; }
  .pick__why { margin: 0; padding-left: 1.1rem; font-size: 0.85rem; color: var(--color-text-muted); }
  .pick__why li:first-child { color: var(--color-text); }
  .lang {
    margin-left: 0.35rem; font-size: 0.7rem; padding: 0 0.4rem; border-radius: 999px; font-weight: 400;
    border: 1px solid var(--color-border); color: var(--color-text-muted); white-space: nowrap;
  }
  .more, .settings { margin-top: 0.8rem; font-size: 0.85rem; }
  .more summary, .settings summary { cursor: pointer; color: var(--color-accent); }
  .runners { font-size: 0.85rem; color: var(--color-text-muted); }
  .ship { display: flex; flex-wrap: wrap; gap: 0.5rem 1rem; align-items: center; font-size: 0.82rem; color: var(--color-text-muted); }
  .ship input { width: 4rem; font: inherit; }
</style>
