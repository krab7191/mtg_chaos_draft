<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { getJSON } from '../../lib/api';
  import { send } from '../../lib/send';
  import { toast } from '../../lib/toast.svelte';
  import { money, pct, age, alertText, evText, contentsTone, type TrendRow, type PricePoint } from '../../lib/trends';
  import { searchLinks } from '../../lib/deals';
  import { withinMax } from '../../lib/filters.svelte';
  import Sparkline from '../trends/Sparkline.svelte';
  import PriceChart from '../trends/PriceChart.svelte';

  interface CheapestResponse {
    products: (TrendRow & { tcgUrl?: string })[];
    dipBelowUsual: number;
    muteAtStock: number;
    notifications: boolean;
  }

  /** before: only sets released before this date (YYYY-MM-DD); '' = every set. */
  let { before: initialBefore = '2020-01-01', canEdit = false }: { before?: string; canEdit?: boolean } = $props();

  let before = $state(initialBefore);
  let data = $state<CheapestResponse | null>(null);
  let loading = $state(true);
  let open = $state<number | null>(null);
  let history = $state<Record<number, PricePoint[]>>({});

  async function load() {
    loading = true;
    const q = before ? `?before=${encodeURIComponent(before)}` : '';
    const res = await getJSON<CheapestResponse>(`/api/cheapest/trends${q}`, "Couldn't load the cheapest packs");
    if (res) data = res;
    loading = false;
  }
  onMount(load);

  async function applyFilter(e: SubmitEvent) {
    e.preventDefault();
    open = null;
    await load();
  }
  async function showAll() {
    before = '';
    open = null;
    await load();
  }

  async function toggle(r: TrendRow) {
    open = open === r.mtgstocksId ? null : r.mtgstocksId;
    if (open != null && !history[open]) {
      const pts = await getJSON<PricePoint[]>(`/api/trends/${r.mtgstocksId}/history`, "Couldn't load price history");
      if (pts) history[r.mtgstocksId] = pts;
    }
    await tick();
  }

  async function setWatch(r: TrendRow, watch: boolean) {
    const ok = await send(`/api/trends/${r.mtgstocksId}/watch`, 'PUT', { watch, dropAlert: null }, "Couldn't update the watch");
    if (!ok) return;
    r.watch = watch ? { mtgstocksId: r.mtgstocksId, dropAlert: null } : null;
    if (watch) {
      const pctUnder = Math.round((data?.dipBelowUsual ?? 0.15) * 100);
      toast.show(`Watching ${r.setName} ${r.productType}: you'll get an alert when it's ${pctUnder}%+ under its usual price or a store lists it well under market`, 'success');
    }
  }

  /** "18% under usual", "5% over usual", or null without a usual price. */
  function vsUsual(r: TrendRow): string | null {
    const v = r.trend?.vsUsual;
    if (v == null) return null;
    const n = Math.round(Math.abs(v) * 100);
    return n === 0 ? 'at usual' : `${n}% ${v < 0 ? 'under' : 'over'} usual`;
  }
</script>

<section class="cheapest" aria-labelledby="cheapest-title">
  <h2 id="cheapest-title" class="cheapest__title">Cheapest packs</h2>
  <p class="cheapest__sub">
    The {data?.products.length ?? 10} cheapest draftable packs by TCGplayer market price. Watch one to get an alert when it dips
    {Math.round((data?.dipBelowUsual ?? 0.15) * 100)}%+ under its usual price (its 6-month median) or a store lists it well under market.
  </p>

  <form class="filter" onsubmit={applyFilter}>
    <label class="filter__label" for="cheapest-before">Released before</label>
    <input class="filter__input" type="date" id="cheapest-before" bind:value={before} />
    <button class="filter__btn" type="submit">Apply</button>
    {#if before}<button class="filter__clear" type="button" onclick={showAll}>All sets</button>{/if}
  </form>

  {#if loading && !data}
    <p class="note">Loading…</p>
  {:else if !data}
    <p class="note">The cheapest packs aren't available right now.</p>
  {:else if data.products.length === 0}
    <p class="note">No packs found.</p>
  {:else}
    <div class="table-wrap">
      <table class="packs">
        <thead>
          <tr>
            <th class="num">#</th>
            <th>Pack</th>
            <th class="num">Price</th>
            <th class="num">vs usual</th>
            <th class="num">30 days</th>
            <th class="num" title="Value of the cards inside (pull rates x card prices), and its 2-week change">Contents</th>
            <th>Trend</th>
            {#if canEdit}<th>Watch</th>{/if}
            <th>Elsewhere</th>
          </tr>
        </thead>
        <tbody>
          {#each data.products.filter(r => withinMax(r.trend?.current ?? r.marketPrice)) as r, i (r.mtgstocksId)}
            {@const t = r.trend}
            <tr id="cheap-{r.mtgstocksId}" class:open={open === r.mtgstocksId} class:alerting={r.alerts.length > 0}>
              <td class="num muted">{i + 1}</td>
              <td class="name">
                <button class="link" aria-expanded={open === r.mtgstocksId} onclick={() => toggle(r)}>{r.setName}</button>
                <span class="sub">{r.productType} · {r.releasedAt.slice(0, 4)} · {age(r.releasedAt)}</span>
                {#if r.alerts.length > 0}
                  <span class="alert-note">{r.alerts.map(a => alertText(r, a)).join('; ')}</span>
                {/if}
              </td>
              <td class="num">
                {money(t?.current ?? r.marketPrice)}
                {#if r.priceDoubt}<span class="doubt" title={r.priceDoubt}>⚠ prices disagree</span>{/if}
              </td>
              <td class="num" class:dip={t?.dip}>{vsUsual(r) ?? '—'}</td>
              <td class="num">{t ? pct(t.change30) : '—'}</td>
              <td class="num">
                {#if evText(r)}
                  <span class="ev tone--{contentsTone(r)}" title="Price vs what the cards inside are worth: green under, plain about equal, yellow to red over">{evText(r)!.value}</span>{#if evText(r)!.change}<span class="sub">{evText(r)!.change} 2 wk</span>{/if}
                {:else}—{/if}
              </td>
              <td>{#if t}<Sparkline values={t.spark} />{/if}</td>
              {#if canEdit}
                <td>
                  <label class="watch">
                    <input type="checkbox" checked={r.watch != null} onchange={async e => {
                      const box = e.currentTarget;
                      await setWatch(r, box.checked);
                      box.checked = r.watch != null;
                    }} />
                    <span class="sr-only">Watch {r.setName} {r.productType}</span>
                  </label>
                </td>
              {/if}
              <td>
                <details class="elsewhere">
                  <summary>Search</summary>
                  <ul>
                    {#each searchLinks(r.setName, r.productType, r.tcgUrl) as l (l.store)}
                      <li><a href={l.url} target="_blank" rel="noopener noreferrer">{l.store}</a></li>
                    {/each}
                  </ul>
                </details>
              </td>
            </tr>
            {#if open === r.mtgstocksId}
              <tr class="detail">
                <td colspan={canEdit ? 9 : 8}>
                  <div class="detail__inner">
                    {#if history[r.mtgstocksId]}
                      <PriceChart
                        points={history[r.mtgstocksId]}
                        released={r.releasedAt}
                        low={t?.usual ?? null}
                        lowLabel="usual"
                        title="{r.setName} {r.productType}: daily market price"
                      />
                    {:else}
                      <p class="note">Loading history…</p>
                    {/if}
                  </div>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<style>
  .cheapest__title { font-size: 1.1rem; margin: 0 0 0.3rem; }
  .cheapest__sub { color: var(--color-text-muted); font-size: 0.85rem; margin: 0 0 0.8rem; }
  .note { font-size: 0.85rem; color: var(--color-text-muted); }

  .filter { display: flex; align-items: center; gap: 0.6rem; margin-bottom: 0.9rem; flex-wrap: wrap; }
  .filter__label { font-size: 0.85rem; color: var(--color-text-muted); }
  .filter__input, .filter__btn {
    padding: 0.3rem 0.6rem; border: 1px solid var(--color-border); border-radius: var(--radius);
    background: var(--color-surface); color: var(--color-text); font: inherit; font-size: 0.85rem;
  }
  .filter__btn { cursor: pointer; }
  .filter__clear { background: none; border: none; color: var(--color-text-muted); font: inherit; font-size: 0.82rem; cursor: pointer; }

  /* position: relative keeps the screen-reader labels inside the scroll box. */
  .table-wrap { position: relative; overflow-x: auto; border: 1px solid var(--color-border); border-radius: var(--radius); }
  .packs { width: 100%; border-collapse: collapse; font-size: 0.85rem; font-variant-numeric: tabular-nums; }
  .packs th {
    text-align: left; font-size: 0.72rem; font-weight: 500; color: var(--color-text-muted);
    background: var(--color-surface); padding: 0.45rem 0.7rem; border-bottom: 1px solid var(--color-border); white-space: nowrap;
  }
  .packs td { padding: 0.45rem 0.7rem; border-bottom: 1px solid var(--color-border); vertical-align: top; white-space: nowrap; }
  .packs .num { text-align: right; }
  .packs tr.alerting td:first-child { box-shadow: inset 3px 0 0 var(--color-success); }
  .packs td.dip { color: var(--color-text); font-weight: 600; }
  .name { white-space: normal !important; min-width: 11rem; }
  .muted { color: var(--color-text-muted); }
  .doubt { display: block; font-size: 0.72rem; color: var(--tone-higher); cursor: help; }
  .sub { display: block; font-size: 0.72rem; color: var(--color-text-muted); }
  .alert-note { display: block; font-size: 0.75rem; color: var(--color-text); margin-top: 0.15rem; }
  .link { background: none; border: none; padding: 0; font: inherit; color: var(--color-text); cursor: pointer; text-align: left; }
  .link:hover, .link:focus-visible { text-decoration: underline; }

  .detail td { background: var(--color-bg); padding: 0.75rem 1rem 1rem; white-space: normal; }
  .detail__inner { position: sticky; left: 1rem; max-width: calc(100vw - 4.25rem); }

  .elsewhere summary { cursor: pointer; color: var(--color-accent); font-size: 0.8rem; }
  .elsewhere ul { list-style: none; padding: 0.3rem 0 0; margin: 0; font-size: 0.8rem; }

  .sr-only {
    position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; border: 0;
    overflow: hidden; clip-path: inset(50%); white-space: nowrap;
  }
</style>
