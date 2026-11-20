<script lang="ts">
  import Spinner from '../Spinner.svelte';
  import { onMount } from 'svelte';
  import { getJSON } from '../../lib/api';
  import { send } from '../../lib/send';
  import { toast } from '../../lib/toast.svelte';
  import { money, pct, valueTone } from '../../lib/trends';
  import {
    filterOffers, applyFilters, groupByPack, searchLinks, languageLabel, checkedAgo,
    type OffersResponse, type OfferView, type StoreOffer,
  } from '../../lib/deals';
  import { filters, setShowOver } from '../../lib/filters.svelte';
  import { watched, seedWatched, markWatched } from '../../lib/watches.svelte';

  let { canEdit = false }: { canEdit?: boolean } = $props();

  let data = $state<OffersResponse | null>(null);
  let loading = $state(true);
  let view = $state<OfferView>('deals');

  // Language and max price come from the shared filter bar above the page.

  onMount(async () => {
    const res = await getJSON<OffersResponse>('/api/store-offers', "Couldn't load store listings");
    if (res) {
      data = res;
      seedWatched(res.watched);
      // Nothing under market today: show what's in stock instead of an empty table.
      if (!res.offers.some(o => o.isDeal)) view = 'instock';
    }
    loading = false;
  });

  const filtered = $derived(data ? applyFilters(data.offers, filters) : []);
  // One row per pack (its best listing), the first PAGE rows unless expanded.
  const PAGE = 25;
  let showAll = $state(false);
  let openGroup = $state<number | null>(null);
  const groups = $derived(groupByPack(filterOffers(filtered, view)));
  const shown = $derived(showAll ? groups : groups.slice(0, PAGE));
  const dealCount = $derived(groupByPack(filterOffers(filtered, 'deals')).length);
  const year = (iso: string) => iso.slice(0, 4);
  const packName = (o: StoreOffer) => (o.productType === 'Booster Pack' ? 'Booster Pack' : o.productType);
  /** The store's own name for it (CardTrader titles carry " · language · ships from …"). */
  const listingName = (o: StoreOffer) => (o.title ? o.title.split(' · ')[0].replace(/^Magic the Gathering:\s*/i, '') : `${o.setName} ${packName(o)}`);

  async function setWatch(o: StoreOffer, watch: boolean) {
    const ok = await send(`/api/trends/${o.mtgstocksId}/watch`, 'PUT', { watch, dropAlert: null }, "Couldn't update the watch");
    if (!ok) return;
    markWatched(o.mtgstocksId, watch);
    if (watch) toast.show(`Watching ${o.setName} ${packName(o)}: you'll get an alert when a store lists it ${pct(-(data?.dealDiscount ?? 0.15))} or more under market`, 'success');
  }
</script>

<section class="deals" aria-labelledby="deals-title">
  <h2 id="deals-title" class="deals__title">Store listings</h2>
  {#if data}
    <p class="deals__sub">
      Single packs at {data.stores.length ? data.stores.join(', ') : 'online stores'} (Card Kingdom and CardTrader checked
      every 2 hours, others daily), compared with TCGplayer's market price. Unreleased sets aren't listed. A <strong>deal</strong> is in stock and {Math.round(data.dealDiscount * 100)}%+ under market;
      watched packs alert your phone when one appears. Foreign-language packs are labelled and compared with the English
      pack's market price. Prices don't include shipping.
    </p>
  {/if}

  {#if loading}
    <Spinner label="Loading store listings…" />
  {:else if !data}
    <p class="note">Store listings aren't available right now.</p>
  {:else if data.offers.length === 0}
    <p class="note">No store listings yet: stores are read once a day, so check back tomorrow.</p>
  {:else}
    <div class="seg" role="group" aria-label="Show">
      <button class:on={view === 'deals'} aria-pressed={view === 'deals'} onclick={() => { view = 'deals'; showAll = false; }}>Deals ({dealCount})</button>
      <button class:on={view === 'instock'} aria-pressed={view === 'instock'} onclick={() => { view = 'instock'; showAll = false; }}>In stock</button>
      <button class:on={view === 'all'} aria-pressed={view === 'all'} onclick={() => { view = 'all'; showAll = false; }}>All listings</button>
    </div>

    <label class="showover">
      <input type="checkbox" checked={filters.showOver} onchange={e => setShowOver(e.currentTarget.checked)} />
      Show listings above market
    </label>

    {#if groups.length === 0}
      <p class="note">{view === 'deals' ? 'No deals right now' : 'Nothing matches'} with these filters. Watch a pack to get an alert when a deal shows up.</p>
    {:else}
      <div class="table-wrap">
        <table class="offers">
          <thead>
            <tr>
              <th>Pack</th>
              <th>Store</th>
              <th class="num">Price</th>
              <th class="num">TCG market</th>
              <th class="num">vs market</th>
              {#if canEdit}<th>Watch</th>{/if}
              <th>Elsewhere</th>
            </tr>
          </thead>
          <tbody>
            {#each shown as g (g.best.mtgstocksId)}
              {@const o = g.best}
              <tr class:deal={o.isDeal} class:soldout={!o.available}>
                <td class="name">
                  <!-- What the store calls it, then what it's priced against, so a wrong match shows. -->
                  {listingName(o)}
                  {#if languageLabel(o.language)}<span class="lang">{languageLabel(o.language)}</span>{/if}
                  <span class="sub">priced as MTGStocks' {o.setName} {packName(o)} · {year(o.releasedAt)}</span>
                  {#if g.others.length > 0}
                    <button type="button" class="more" aria-expanded={openGroup === o.mtgstocksId}
                      onclick={() => (openGroup = openGroup === o.mtgstocksId ? null : o.mtgstocksId)}>
                      {openGroup === o.mtgstocksId ? 'Hide' : `+${g.others.length} more`} listing{g.others.length === 1 ? '' : 's'}
                    </button>
                    {#if openGroup === o.mtgstocksId}
                      <ul class="others">
                        {#each g.others as x (x.store + x.url)}
                          <li>
                            <a href={x.url} target="_blank" rel="noopener noreferrer">{x.store}</a>
                            {#if languageLabel(x.language)}<span class="lang">{languageLabel(x.language)}</span>{/if}
                            {money(x.price)} · {x.discount > 0 ? `${Math.round(x.discount * 100)}% under` : x.discount < 0 ? `${Math.round(-x.discount * 100)}% over` : 'at market'}{x.available ? '' : ' · sold out'}
                          </li>
                        {/each}
                      </ul>
                    {/if}
                  {/if}
                </td>
                <td>
                  <a href={o.url} target="_blank" rel="noopener noreferrer">{o.store}</a>
                  <span class="sub">{o.available ? 'In stock' : 'Sold out'}{o.seenAt ? ` · ${checkedAgo(o.seenAt)}` : ''}</span>
                </td>
                <td class="num">
                  {money(o.price)}
                  {#if o.contents}
                    <!-- Coloured by price vs what the cards inside are worth (one scale for all packs). -->
                    <span class="contents-note tone--{valueTone(o.price, o.contents)}"
                      title="Value of the cards inside (pull rates × card prices). Green: price under it; yellow to red: over">
                      {o.premium ? `⚠ ${(o.price / o.contents).toFixed(1)}× contents` : `contents ${money(o.contents)}`}
                    </span>
                  {/if}
                </td>
                <td class="num">{money(o.marketPrice)}</td>
                <td class="num">
                  {o.discount > 0 ? `${Math.round(o.discount * 100)}% under` : o.discount < 0 ? `${Math.round(-o.discount * 100)}% over` : 'at market'}
                  {#if o.suspicious}<span class="contents-note doubt" title="An English pack this far under market is more often a mislisting or a market price that's off than a bargain">⚠ unusually cheap, check it</span>{/if}
                </td>
                {#if canEdit}
                  <td>
                    <label class="watch">
                      <input type="checkbox" checked={watched.has(o.mtgstocksId)} onchange={async e => {
                        const box = e.currentTarget;
                        await setWatch(o, box.checked);
                        box.checked = watched.has(o.mtgstocksId);
                      }} />
                      <span class="sr-only">Watch {o.setName} {packName(o)}</span>
                    </label>
                  </td>
                {/if}
                <td>
                  <details class="elsewhere">
                    <summary>Search</summary>
                    <ul>
                      {#each searchLinks(o.setName, o.productType, o.tcgUrl) as l (l.store)}
                        <li><a href={l.url} target="_blank" rel="noopener noreferrer">{l.store}</a></li>
                      {/each}
                    </ul>
                  </details>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if groups.length > shown.length}
        <button type="button" class="show-all" onclick={() => (showAll = true)}>Show all {groups.length}</button>
      {/if}
    {/if}
  {/if}
</section>

<style>
  .deals { margin-bottom: 2rem; }
  .deals__title { font-size: 1.1rem; margin: 0 0 0.3rem; }
  .deals__sub { color: var(--color-text-muted); font-size: 0.85rem; margin: 0 0 0.8rem; }
  .note { font-size: 0.85rem; color: var(--color-text-muted); }

  .seg { display: inline-flex; border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; margin-bottom: 0.8rem; }
  .seg button {
    background: none; border: none; color: var(--color-text-muted); font: inherit; font-size: 0.85rem;
    padding: 0.35rem 0.75rem; cursor: pointer;
  }
  .seg button + button { border-left: 1px solid var(--color-border); }
  .seg button.on { background: var(--color-surface); color: var(--color-text); font-weight: 600; }

  /* position: relative keeps the screen-reader labels inside the scroll box. */
  .table-wrap { position: relative; overflow-x: auto; border: 1px solid var(--color-border); border-radius: var(--radius); }
  .offers { width: 100%; border-collapse: collapse; font-size: 0.85rem; font-variant-numeric: tabular-nums; }
  .offers th {
    text-align: left; font-size: 0.72rem; font-weight: 500; color: var(--color-text-muted);
    background: var(--color-surface); padding: 0.45rem 0.7rem; border-bottom: 1px solid var(--color-border); white-space: nowrap;
  }
  .offers td { padding: 0.45rem 0.7rem; border-bottom: 1px solid var(--color-border); vertical-align: top; white-space: nowrap; }
  .offers .num { text-align: right; }
  .offers tr.deal td:first-child { box-shadow: inset 3px 0 0 var(--color-success); }
  .offers tr.soldout td { color: var(--color-text-muted); }
  .name { white-space: normal !important; min-width: 10rem; }
  .sub { display: block; font-size: 0.72rem; color: var(--color-text-muted); }
  /* Like .sub but coloured by its tone (a .sub colour here would override it). */
  .contents-note { display: block; font-size: 0.72rem; }
  .doubt { color: var(--tone-higher); cursor: help; }
  .more { background: none; border: none; padding: 0; margin-top: 0.15rem; font: inherit; font-size: 0.72rem; color: var(--color-accent); cursor: pointer; }
  .others { list-style: none; margin: 0.2rem 0 0; padding: 0; font-size: 0.75rem; color: var(--color-text-muted); }
  .others li { padding: 0.1rem 0; }
  .show-all { margin-top: 0.6rem; background: none; border: 1px solid var(--color-border); border-radius: var(--radius);
    color: var(--color-text); font: inherit; font-size: 0.82rem; padding: 0.3rem 0.8rem; cursor: pointer; }
  .showover { display: inline-flex; gap: 0.3rem; align-items: center; cursor: pointer; margin: 0 0 0.8rem 1rem; font-size: 0.82rem; color: var(--color-text-muted); }
  .lang {
    margin-left: 0.35rem; font-size: 0.7rem; padding: 0 0.4rem; border-radius: 999px;
    border: 1px solid var(--color-border); color: var(--color-text-muted); white-space: nowrap;
  }

  .elsewhere summary { cursor: pointer; color: var(--color-accent); font-size: 0.8rem; }
  .elsewhere ul { list-style: none; padding: 0.3rem 0 0; margin: 0; font-size: 0.8rem; }
  .elsewhere li { padding: 0.1rem 0; }

  .sr-only {
    position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; border: 0;
    overflow: hidden; clip-path: inset(50%); white-space: nowrap;
  }
</style>
