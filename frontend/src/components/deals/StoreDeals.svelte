<script lang="ts">
  import { onMount } from 'svelte';
  import { getJSON } from '../../lib/api';
  import { send } from '../../lib/send';
  import { toast } from '../../lib/toast.svelte';
  import { money, pct } from '../../lib/trends';
  import { filterOffers, searchLinks, languageLabel, type OffersResponse, type OfferView, type StoreOffer } from '../../lib/deals';

  let { canEdit = false }: { canEdit?: boolean } = $props();

  let data = $state<OffersResponse | null>(null);
  let loading = $state(true);
  let view = $state<OfferView>('deals');
  let watched = $state(new Set<number>());

  onMount(async () => {
    const res = await getJSON<OffersResponse>('/api/store-offers', "Couldn't load store listings");
    if (res) {
      data = res;
      watched = new Set(res.watched);
      // Nothing under market today: show what's in stock instead of an empty table.
      if (!res.offers.some(o => o.isDeal)) view = 'instock';
    }
    loading = false;
  });

  const rows = $derived(data ? filterOffers(data.offers, view) : []);
  const dealCount = $derived(data ? data.offers.filter(o => o.isDeal).length : 0);
  const year = (iso: string) => iso.slice(0, 4);
  const packName = (o: StoreOffer) => (o.productType === 'Booster Pack' ? 'Booster Pack' : o.productType);

  async function setWatch(o: StoreOffer, watch: boolean) {
    const ok = await send(`/api/trends/${o.mtgstocksId}/watch`, 'PUT', { watch, dropAlert: null }, "Couldn't update the watch");
    if (!ok) return;
    const next = new Set(watched);
    if (watch) next.add(o.mtgstocksId); else next.delete(o.mtgstocksId);
    watched = next;
    if (watch) toast.show(`Watching ${o.setName} ${packName(o)}: you'll get an alert when a store lists it ${pct(-(data?.dealDiscount ?? 0.15))} or more under market`, 'success');
  }
</script>

<section class="deals" aria-labelledby="deals-title">
  <h2 id="deals-title" class="deals__title">Store listings</h2>
  {#if data}
    <p class="deals__sub">
      Single packs at {data.stores.length ? data.stores.join(', ') : 'online stores'}, checked daily and compared with
      TCGplayer's market price. A <strong>deal</strong> is in stock and {Math.round(data.dealDiscount * 100)}%+ under market;
      watched packs alert your phone when one appears. Foreign-language packs are labelled and compared with the English
      pack's market price. Prices don't include shipping.
    </p>
  {/if}

  {#if loading}
    <p class="note">Loading…</p>
  {:else if !data}
    <p class="note">Store listings aren't available right now.</p>
  {:else if data.offers.length === 0}
    <p class="note">No store listings yet: stores are read once a day, so check back tomorrow.</p>
  {:else}
    <div class="seg" role="group" aria-label="Show">
      <button class:on={view === 'deals'} aria-pressed={view === 'deals'} onclick={() => (view = 'deals')}>Deals ({dealCount})</button>
      <button class:on={view === 'instock'} aria-pressed={view === 'instock'} onclick={() => (view = 'instock')}>In stock</button>
      <button class:on={view === 'all'} aria-pressed={view === 'all'} onclick={() => (view = 'all')}>All listings</button>
    </div>

    {#if rows.length === 0}
      <p class="note">No deals right now. Watch a pack to get an alert when one shows up.</p>
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
            {#each rows as o (o.store + o.url)}
              <tr class:deal={o.isDeal} class:soldout={!o.available}>
                <td class="name">
                  {o.setName}
                  {#if languageLabel(o.language)}<span class="lang">{languageLabel(o.language)}</span>{/if}
                  <span class="sub">{packName(o)} · {year(o.releasedAt)}</span>
                </td>
                <td>
                  <a href={o.url} target="_blank" rel="noopener noreferrer">{o.store}</a>
                  <span class="sub">{o.available ? 'In stock' : 'Sold out'}{o.preorder ? ' · Preorder' : ''}</span>
                </td>
                <td class="num">
                  {money(o.price)}
                  {#if o.contents}
                    <span class="sub" class:warn={o.premium} title="Value of the cards inside (pull rates × card prices)">
                      {o.premium ? `⚠ ${(o.price / o.contents).toFixed(1)}× contents` : `contents ${money(o.contents)}`}
                    </span>
                  {/if}
                </td>
                <td class="num">{money(o.marketPrice)}</td>
                <td class="num">
                  {o.discount > 0 ? `${Math.round(o.discount * 100)}% under` : o.discount < 0 ? `${Math.round(-o.discount * 100)}% over` : 'at market'}
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
  .sub.warn { color: var(--color-danger); }
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
