<script lang="ts">
  import { onMount, tick } from 'svelte';
  import Sparkline from './Sparkline.svelte';
  import PriceChart from './PriceChart.svelte';
  import { getJSON } from '../../lib/api';
  import { send } from '../../lib/send';
  import { toast } from '../../lib/toast.svelte';
  import {
    SIGNALS, evText, contentsTone, filterRows, YEAR_OPTIONS, yearsLabel, windowFor, isBox, perPack, money, pct, age, alertText, alertsFor, watchFeedback, stockNote, describeRule, groupBySet, MUTE_AT_STOCK, mutedStock,
    type TrendRow, type TrendsResponse, type PricePoint, type KindFilter, type ShowFilter, type RuleProposal, type BTMetrics,
  } from '../../lib/trends';

  let { canEdit = false }: { canEdit?: boolean } = $props();

  let data = $state<TrendsResponse | null>(null);
  let loading = $state(true);
  let years = $state(2);
  let kind = $state<KindFilter>('box');
  let show = $state<ShowFilter>('all');
  let open = $state<number | null>(null);
  let history = $state<Record<number, PricePoint[]>>({});

  async function load() {
    loading = true;
    const res = await getJSON<TrendsResponse>(`/api/trends?years=${years}`, "Couldn't load price trends");
    if (res) data = res;
    loading = false;
  }

  // The list always includes watched packs, however old. So the window opens
  // wide enough to cover the oldest one, and the dropdown says what's shown.
  onMount(async () => {
    await load();
    const needed = windowFor(data?.products ?? []);
    if (needed > years) {
      years = needed;
      await load();
    }
  });

  const rows = $derived(data ? filterRows(data.products, kind, show) : []);
  const alerting = $derived(data ? data.products.filter(r => r.alerts.length > 0) : []);
  const alertGroups = $derived(groupBySet(alerting));
  const ALERTS_OPEN_KEY = 'trendsAlertsOpen';
  let alertsOpen = $state((() => {
    try { return localStorage.getItem(ALERTS_OPEN_KEY) !== 'false'; } catch { return true; }
  })());
  function rememberAlertsOpen(isOpen: boolean) {
    alertsOpen = isOpen;
    try { localStorage.setItem(ALERTS_OPEN_KEY, String(isOpen)); } catch { /* private mode: not remembered */ }
  }
  // Leave "Needs a look" once nothing alerts (its button is hidden then).
  $effect(() => { if (show === 'alerting' && alerting.length === 0) show = 'all'; });
  const th = $derived(data?.thresholds);

  async function toggle(r: TrendRow, keepOpen = false) {
    open = keepOpen || open !== r.mtgstocksId ? r.mtgstocksId : null;
    if (open != null && !history[open]) {
      const pts = await getJSON<PricePoint[]>(`/api/trends/${r.mtgstocksId}/history`, "Couldn't load price history");
      if (pts) history[r.mtgstocksId] = pts;
    }
  }

  // "Needs a look" spans boxes and packs; picking a kind goes back to all of that kind.
  function pickKind(k: KindFilter) {
    kind = k;
    if (show === 'alerting') show = 'all';
  }

  /** From an alert: show the product's row in the table, scroll to it and open its graph. */
  async function reveal(r: TrendRow) {
    if (show !== 'alerting') { // "Needs a look" already lists it
      kind = isBox(r) ? 'box' : 'pack';
      show = 'all';
    }
    const loading = toggle(r, true);
    await tick();
    document.getElementById(`trend-${r.mtgstocksId}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    await loading;
  }

  async function setWatch(r: TrendRow, watch: boolean, dropAlert: number | null = r.watch?.dropAlert ?? null) {
    const ok = await send(`/api/trends/${r.mtgstocksId}/watch`, 'PUT', { watch, dropAlert }, "Couldn't update the watch");
    if (!ok) return;
    const wasWatched = r.watch != null;
    const hadAlert = r.alerts.length > 0;
    r.watch = watch ? { mtgstocksId: r.mtgstocksId, dropAlert } : null;
    // Recompute this row's alerts locally; the next load confirms them.
    r.alerts = alertsFor(r.trend, r.watch, r.stock, r.deal);
    // Say right away whether it would alert: on a new watch, or when a
    // changed drop alert starts firing.
    if (watch && (!wasWatched || (!hadAlert && r.alerts.length > 0))) {
      const f = watchFeedback(r);
      toast.show(f.msg, f.alerting ? 'success' : 'info');
    }
  }

  // ── Watch / unwatch every row shown ──────────────────────────
  const allWatched = $derived(rows.length > 0 && rows.every(r => r.watch != null));
  const someWatched = $derived(rows.some(r => r.watch != null));

  async function setWatchAll(watch: boolean) {
    const targets = rows.filter(r => (r.watch != null) !== watch);
    if (targets.length === 0) return;
    const ok = await send('/api/trends/watch', 'PUT', { ids: targets.map(r => r.mtgstocksId), watch }, "Couldn't update the watches");
    if (!ok) return;
    for (const r of targets) {
      r.watch = watch ? { mtgstocksId: r.mtgstocksId, dropAlert: null } : null;
      r.alerts = alertsFor(r.trend, r.watch, r.stock, r.deal);
    }
    if (watch) {
      const alerting = targets.filter(r => r.alerts.length > 0).length;
      toast.show(
        `Watching ${targets.length}${alerting ? ` · ${alerting} already alerting (see the top of the page)` : ' · none alerting right now'}`,
        alerting ? 'success' : 'info',
      );
    } else {
      toast.show(`Stopped watching ${targets.length}`, 'info');
    }
  }

  let testing = $state(false);
  async function sendTest() {
    testing = true;
    if (await send('/api/trends/notify-test', 'POST', null, "Test alert failed")) {
      toast.show('Test alert sent: check your phone', 'success');
    }
    testing = false;
  }

  // ── Floor rule proposals from the quarterly backtest ─────────
  const kindName = (k: string) => (k === 'collector' ? 'Collector packs & boxes' : 'Regular packs & boxes');
  const pct0 = (f: number) => `${Math.round(f * 100)}%`;
  const resultRows = (m: BTMetrics) => [
    ['Alerts within 5% of the eventual low', pct0(m.hit5 ?? 0)],
    ['Alerts within 10% of the eventual low', pct0(m.hit10 ?? 0)],
    ['Bought above the eventual low (median)', pct0(m.median)],
    ['Bought above the eventual low (average)', pct0(m.mean)],
    ['Fired on', `${pct0(m.coverage)} of products`],
    ['Price fell another 10%+ afterwards', pct0(m.falseRate)],
  ];
  let deciding = $state(false);
  async function decide(p: RuleProposal, apply: boolean) {
    deciding = true;
    const ok = await send(`/api/trends/rules/${p.id}/${apply ? 'apply' : 'dismiss'}`, 'POST', null,
      apply ? "Couldn't apply the rule" : "Couldn't dismiss the proposal");
    deciding = false;
    if (!ok) return;
    toast.show(apply ? `New ${kindName(p.kind).toLowerCase()} rule applied` : 'Proposal dismissed', 'success');
    await load(); // signals recompute with the active rules
  }
  let running = $state(false);
  async function runBacktest() {
    running = true;
    const ok = await send('/api/trends/backtest', 'POST', null, 'Backtest failed');
    running = false;
    if (!ok) return;
    await load();
    toast.show(data?.proposals?.length ? 'Backtest done: a better rule was found, see the top of the page' : 'Backtest done: the current rules are still the best', 'success');
  }

  /** Mixed selection shows as the checkbox's in-between state. */
  function indeterminate(node: HTMLInputElement, value: boolean) {
    node.indeterminate = value;
    return { update(v: boolean) { node.indeterminate = v; } };
  }

  function onDropInput(r: TrendRow, value: string) {
    const n = Number(value);
    const dropAlert = value.trim() === '' ? null : n > 0 && n < 100 ? n / 100 : undefined;
    if (dropAlert === undefined) return; // ignore out-of-range input
    void setWatch(r, true, dropAlert);
  }

  const kindLabel = (r: TrendRow) => r.productType;
</script>

<!-- ── Proposed floor rules ──────────────────────────────── -->
{#each data?.proposals ?? [] as p (p.id)}
  <section class="proposal" aria-label="Proposed floor rule">
    <h2 class="proposal__title">The quarterly backtest found a better floor rule for {kindName(p.kind).toLowerCase()}</h2>
    <table class="proposal__table">
      <thead><tr><th></th><th>Current</th><th>Proposed</th></tr></thead>
      <tbody>
        <tr><td>Rule</td><td>{describeRule(p.current)}</td><td>{describeRule(p.rule)}</td></tr>
        {#if p.metrics}
          {@const cur = resultRows(p.metrics.current)}
          {@const prop = resultRows(p.metrics.proposed)}
          {#each cur as [label, value], i (label)}
            <tr><td>{label}</td><td>{value}</td><td>{prop[i][1]}</td></tr>
          {/each}
        {/if}
      </tbody>
    </table>
    {#if p.metrics}<p class="note">Tested on {p.metrics.products} products, each scored by a rule tuned without its release year.</p>{/if}
    {#if canEdit}
      <div class="proposal__actions">
        <button class="btn-apply" disabled={deciding} onclick={() => decide(p, true)}>Apply</button>
        <button class="link link--btn" disabled={deciding} onclick={() => decide(p, false)}>Dismiss</button>
      </div>
    {/if}
  </section>
{/each}

<!-- ── Alerts ─────────────────────────────────────────────── -->
{#if alerting.length > 0}
  <!-- Collapsible (it gets long); open or closed is remembered in this browser. -->
  <details class="alerts" aria-label="Alerts" open={alertsOpen} ontoggle={e => rememberAlertsOpen(e.currentTarget.open)}>
    <summary class="alerts__summary">
      <h2 class="alerts__title">
        ◆ {alerting.length} watched {alerting.length === 1 ? 'product' : 'products'}{alertGroups.length > 1 ? ` in ${alertGroups.length} sets` : ''}
        {alerting.length === 1 ? 'needs' : 'need'} a look
      </h2>
    </summary>
    {#each alertGroups as g (g.setName)}
      <h3 class="alerts__set">{g.setName}</h3>
      <ul>
        {#each g.rows as r (r.mtgstocksId)}
          <li>
            <button class="link" onclick={() => reveal(r)}>
              {kindLabel(r)}
            </button>
            — {r.alerts.map(a => alertText(r, a)).join('; ')}
            {#if stockNote(r)}<span class="stock-note">{stockNote(r)}</span>{/if}
          </li>
        {/each}
      </ul>
    {/each}
  </details>
{/if}

<!-- ── Filters ────────────────────────────────────────────── -->
<div class="filters">
  <div class="seg" role="group" aria-label="Product kind">
    <button class:on={kind === 'box' && show !== 'alerting'} aria-pressed={kind === 'box' && show !== 'alerting'} onclick={() => pickKind('box')}>Booster boxes</button>
    <button class:on={kind === 'pack' && show !== 'alerting'} aria-pressed={kind === 'pack' && show !== 'alerting'} onclick={() => pickKind('pack')}>Single packs</button>
  </div>
  <div class="seg" role="group" aria-label="Show">
    <button class:on={show === 'all'} aria-pressed={show === 'all'} onclick={() => (show = 'all')}>All</button>
    {#if alerting.length > 0}
      <button class:on={show === 'alerting'} aria-pressed={show === 'alerting'} onclick={() => (show = 'alerting')}>Needs a look ({alerting.length})</button>
    {/if}
    <button class:on={show === 'floor'} aria-pressed={show === 'floor'} onclick={() => (show = 'floor')}>◆ Possible floors</button>
    <button class:on={show === 'watched'} aria-pressed={show === 'watched'} onclick={() => (show = 'watched')}>Watched</button>
  </div>
  <label class="years">
    Released in the last
    <select bind:value={years} onchange={load}>
      {#each YEAR_OPTIONS as y (y)}<option value={y}>{yearsLabel(y)}</option>{/each}
    </select>
  </label>
  {#if data && data.backfilled < data.tracked}
    <span class="history" title="Products with price history loaded">History {data.backfilled}/{data.tracked}</span>
  {/if}
</div>

{#if data && canEdit}
  <p class="phone">
    {#if data.notifications}
      📱 Phone alerts are on: watched products notify you through ntfy when a new alert appears (checked daily).
      <button class="link link--btn" onclick={sendTest} disabled={testing}>{testing ? 'Sending…' : 'Send test alert'}</button>
    {:else}
      📱 Phone alerts are off. Install the free ntfy app, subscribe to a long random topic, and set
      <code>NTFY_URL=https://ntfy.sh/&lt;topic&gt;</code> in the server's <code>.env</code>.
    {/if}
  </p>
{/if}

<!-- ── Table ──────────────────────────────────────────────── -->
{#if loading && !data}
  <p class="note">Loading…</p>
{:else if rows.length === 0}
  <p class="note">Nothing matches these filters.</p>
{:else}
  <div class="table-wrap">
    <table class="trends">
      <thead>
        <tr>
          <th>Set</th>
          <th class="num">Price</th>
          <th class="num">Since release</th>
          <th class="num">Low since release</th>
          <th class="num">{th?.trendWindow ?? 30} days</th>
          <th class="num" title="Value of the cards inside (pull rates x card prices), and its 2-week change">Contents</th>
          <th>Signal</th>
          <th>Trend</th>
          {#if canEdit}
            <th>
              <label class="watch__all" title={allWatched ? 'Stop watching every row shown' : 'Watch every row shown'}>
                <input
                  type="checkbox"
                  checked={allWatched}
                  use:indeterminate={someWatched && !allWatched}
                  onchange={async e => {
                    const box = e.currentTarget;
                    await setWatchAll(!allWatched);
                    box.checked = allWatched; // snap back if saving failed
                  }}
                />
                Watch
              </label>
            </th>
          {/if}
        </tr>
      </thead>
      <tbody>
        {#each rows as r (r.mtgstocksId)}
          {@const t = r.trend}
          {@const s = t ? SIGNALS[t.signal] : null}
          <tr id="trend-{r.mtgstocksId}" class:open={open === r.mtgstocksId} class:alerting={r.alerts.length > 0}>
            <td class="name">
              <button class="link" aria-expanded={open === r.mtgstocksId} onclick={() => toggle(r)}>{r.setName}</button>
              <span class="muted">{kindLabel(r)}</span>
              <span class="sub">{r.releasedAt} · {age(r.releasedAt)}</span>
            </td>
            <td class="num">
              {money(t?.current ?? r.marketPrice)}
              {#if isBox(r)}<span class="sub">{money(perPack(r))} / pack · {r.packsPerBox} packs</span>{/if}
              {#if r.priceDoubt}<span class="doubt" title={r.priceDoubt}>⚠ prices disagree</span>{/if}
            </td>
            <!-- Preorders have no release price yet: their change is only since listing. -->
            <td class="num" title={t?.signal === 'preorder' ? 'Not released yet' : undefined}>
              {t && t.signal !== 'preorder' ? pct(-t.dropFromRelease) : '—'}
            </td>
            <td class="num">
              {t ? money(t.low) : '—'}
              {#if t}<span class="sub">{t.aboveLow < 0.005 ? 'at the low' : `${pct(t.aboveLow)} above`}</span>{/if}
            </td>
            <td class="num">{t ? pct(t.change30) : '—'}</td>
            <td class="num">
              {#if evText(r)}
                <span class="ev tone--{contentsTone(r)}" title="Price vs what the cards inside are worth: green under, plain about equal, yellow to red over">{evText(r)!.value}</span>{#if evText(r)!.change}<span class="sub">{evText(r)!.change} 2 wk</span>{/if}
              {:else}—{/if}
            </td>
            <td>
              {#if s}
                <span class="signal signal--{t!.signal}" title={s.hint}><span aria-hidden="true">{s.icon}</span> {s.label}</span>
                {#if t!.signal === 'climbing' && t!.change365 != null}<span class="sub">{pct(t!.change365)} in a year</span>{/if}
                {#if mutedStock(r) != null}
                  <span class="sub" title="Alerts are muted once you have {MUTE_AT_STOCK}+ packs">alert muted · you own {mutedStock(r)}</span>
                {/if}
              {:else}
                <span class="muted">no history yet</span>
              {/if}
            </td>
            <td>{#if t}<Sparkline values={t.spark} />{/if}</td>
            {#if canEdit}
              <td class="watch">
                <label class="watch__box">
                  <input type="checkbox" checked={r.watch != null} onchange={async e => {
                    const box = e.currentTarget;
                    await setWatch(r, box.checked, null);
                    box.checked = r.watch != null; // snap back if saving failed
                  }} />
                  <span class="sr-only">Watch {r.setName} {kindLabel(r)}</span>
                </label>
                {#if r.watch}
                  <label class="watch__drop" title="Also alert when the price is this far below its release price">
                    <input
                      type="number" min="1" max="99" step="1" inputmode="numeric" placeholder="−%"
                      value={r.watch.dropAlert != null ? Math.round(r.watch.dropAlert * 100) : ''}
                      onchange={e => onDropInput(r, (e.target as HTMLInputElement).value)}
                    />
                    <span class="sr-only">Drop alert, percent below release</span>
                  </label>
                {/if}
              </td>
            {/if}
          </tr>
          {#if open === r.mtgstocksId}
            <tr class="detail">
              <td colspan={canEdit ? 9 : 8}>
                <div class="detail__inner">
                  {#if history[r.mtgstocksId]}
                    <PriceChart
                      points={history[r.mtgstocksId]}
                      released={r.releasedAt}
                      low={t?.low ?? null}
                      perPackDivisor={r.packsPerBox ?? 1}
                      title="{r.setName} {kindLabel(r)}: daily market price"
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

{#if th}
  <details class="explain">
    <summary>How the signals work</summary>
    <ul>
      <li><strong>◆ Possible floor</strong>, regular packs and boxes: {describeRule(th.floorRules.regular)}.</li>
      <li><strong>◆ Possible floor</strong>, Collector packs and boxes: {describeRule(th.floorRules.collector)}.
        Collector prices often don't drop much after release, so this rule doesn't require a drop.</li>
      <li>Both rules are backtested on real price histories (sets 2019–2026), each tested on release years it
        wasn't tuned on, and tuned for alerts that land within 5% of the eventual low. Regular: ~31% of alerts land
        within 5% of the low and ~50% within 10% (median ~10% above it); waiting a fixed 3 months lands within 5% only ~6%
        of the time for boxes. Collector: ~26% within 5%. It's a strong hint, not a guarantee. Watched products that hit
        a floor raise an alert.</li>
      <li>Alerts are muted for products you already have {MUTE_AT_STOCK}+ packs of (counting loose packs of the
        same set and kind); the signal still shows in the table.</li>
      <li><strong>▽ Below usual</strong> (watched packs 2+ years old): the week's median price is 15%+ under its usual
        price (the median over the 6 months before the last 2 weeks). Older packs mostly drift up, so this means "cheaper than
        usual right now", not "the bottom": in tests the price kept sliding about half the time.</li>
      <li><strong>$ Store deal</strong> (watched packs, once released): a store lists it in stock 15%+ under TCGplayer's
        market price (see Cheap Packs). Preorder listings don't count: before release the market price is preorder hype.</li>
      <li><strong>↗ Climbing</strong>: a year or more after release, up 25%+ over the past year and not falling over the
        last 30 days. In tests such packs were higher 6 months later 85–93% of the time (median +16–26%), and only about
        1 in 10 dipped 10%+ below the price meanwhile: if you want one, waiting for a dip has rarely paid.</li>
      <li><strong>Contents</strong>: what the cards inside a pack are worth on average (for a box, its packs' value added up), from MTGJSON's pull rates
        and TCGplayer card prices, and how that moved over 2 weeks. When the cards are heating up (spoilers, new decks),
        packs have tended to follow over the next few weeks; when they cool off, waiting tends to pay. In a 90-day test the
        effect was a few percent, so it's a hint, not an alert. Its colour compares the price with the contents, on one scale
        for every pack: dark green 20%+ under, light green 1–20% under, plain within 1%, yellow up to 25% over, orange up to
        2×, red over 2×. Collector packs are mostly orange or red: they usually sell well above their cards.</li>
      <li>Not tracked here: Jumpstart (casual half-decks, not drafted) and sleeved retail packs (gone weeks after
        release). Both can still be added to the collection.</li>
      <li><strong>▼ New low</strong> (watched products): the week's median price broke at least 2% below its previous low,
        once a product has 2+ months of history. One cheap sale can't trigger it.</li>
      <li>The rules are re-tested every quarter on the latest price history; a clearly better rule is proposed here
        for you to apply.
        {#if data?.lastBacktest}Last run: {new Date(data.lastBacktest).toLocaleDateString()}.{:else}Not run yet.{/if}
        {#if canEdit}
          <button class="link link--btn" onclick={runBacktest} disabled={running}>{running ? 'Running…' : 'Run backtest now'}</button>
        {/if}
      </li>
      <li><strong>↓ Falling / ↑ Rising</strong>: moved more than {Math.round(th.trendMove * 100)}% over {th.trendWindow} days.</li>
      <li><strong>★ New</strong>: released under {th.newSetDays} days ago, when prices still carry release hype.</li>
      <li>Prices are TCGplayer market prices via MTGStocks. A box's per-pack price assumes its standard pack count.</li>
    </ul>
  </details>
{/if}

<style>
  /* Rows scrolled to from an alert clear the sticky nav. */
  tbody tr { scroll-margin-top: 4rem; }
  .proposal {
    border: 1px solid var(--color-accent);
    border-radius: var(--radius);
    padding: 0.75rem 1rem;
    margin-bottom: 1rem;
    font-size: 0.85rem;
  }
  .proposal__title { font-size: 0.95rem; margin: 0 0 0.5rem; }
  .proposal__table { width: 100%; border-collapse: collapse; }
  .proposal__table th, .proposal__table td {
    text-align: left; vertical-align: top; padding: 0.3rem 0.5rem; border-bottom: 1px solid var(--color-border);
  }
  .proposal__table th { font-size: 0.72rem; color: var(--color-text-muted); font-weight: 500; }
  .proposal__table td:first-child { color: var(--color-text-muted); white-space: nowrap; }
  .proposal__actions { display: flex; gap: 0.75rem; align-items: center; margin-top: 0.6rem; }
  .btn-apply {
    background: var(--color-accent); color: var(--color-text); border: none; border-radius: var(--radius);
    padding: 0.35rem 0.9rem; font: inherit; cursor: pointer;
  }
  .btn-apply:disabled { opacity: 0.6; cursor: default; }

  .alerts {
    border: 1px solid var(--color-success);
    border-radius: var(--radius);
    padding: 0.75rem 1rem;
    margin-bottom: 1rem;
  }
  .alerts__summary { cursor: pointer; }
  .alerts__title { display: inline; font-size: 0.95rem; margin: 0; }
  .alerts[open] .alerts__summary { margin-bottom: 0.4rem; }
  .alerts ul { margin: 0 0 0.35rem; padding-left: 1.1rem; font-size: 0.85rem; }
  .alerts__set { font-size: 0.85rem; font-weight: 600; margin: 0.5rem 0 0.15rem; }
  .stock-note {
    margin-left: 0.4rem; font-size: 0.75rem; padding: 0 0.4rem; border-radius: 999px;
    border: 1px solid var(--color-border); color: var(--color-text-muted); white-space: nowrap;
  }

  .filters { display: flex; flex-wrap: wrap; gap: 0.6rem 1rem; align-items: center; margin-bottom: 0.9rem; }
  .seg { display: inline-flex; border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
  .seg button {
    background: none; border: none; color: var(--color-text-muted); font: inherit; font-size: 0.85rem;
    padding: 0.35rem 0.75rem; cursor: pointer;
  }
  .seg button + button { border-left: 1px solid var(--color-border); }
  .seg button.on { background: var(--color-surface); color: var(--color-text); font-weight: 600; }
  .years { font-size: 0.85rem; color: var(--color-text-muted); display: inline-flex; gap: 0.4rem; align-items: center; }
  .years select { font: inherit; }
  .history { font-size: 0.8rem; color: var(--color-text-muted); margin-left: auto; font-variant-numeric: tabular-nums; }

  .note { font-size: 0.85rem; color: var(--color-text-muted); }
  .phone { font-size: 0.82rem; color: var(--color-text-muted); margin: 0 0 0.9rem; }
  .phone code { font-size: 0.78rem; }
  .link--btn { color: var(--color-accent); margin-left: 0.4rem; }

  /* position: relative keeps absolutely positioned bits (screen-reader labels) inside the
     scroll box; otherwise they sit at the table's full width and widen the page on phones. */
  .table-wrap { position: relative; overflow-x: auto; border: 1px solid var(--color-border); border-radius: var(--radius); }
  .trends { width: 100%; border-collapse: collapse; font-size: 0.85rem; font-variant-numeric: tabular-nums; }
  .trends th {
    text-align: left; font-size: 0.72rem; font-weight: 500; color: var(--color-text-muted);
    background: var(--color-surface); padding: 0.45rem 0.7rem; border-bottom: 1px solid var(--color-border); white-space: nowrap;
  }
  .trends td { padding: 0.45rem 0.7rem; border-bottom: 1px solid var(--color-border); white-space: nowrap; vertical-align: middle; }
  .trends .num { text-align: right; }
  .trends tr.alerting td:first-child { box-shadow: inset 3px 0 0 var(--color-success); }
  .name { white-space: normal !important; min-width: 11rem; }
  .muted { color: var(--color-text-muted); font-size: 0.85em; white-space: nowrap; }
  .sub { display: block; font-size: 0.7rem; color: var(--color-text-muted); }
  .link { background: none; border: none; padding: 0; font: inherit; color: var(--color-text); cursor: pointer; text-align: left; }
  .link:hover, .link:focus-visible { text-decoration: underline; }

  .signal { font-size: 0.78rem; padding: 0.1rem 0.45rem; border-radius: 999px; border: 1px solid var(--color-border); white-space: nowrap; }
  .signal--floor { border-color: var(--color-success); color: var(--color-text); font-weight: 600; }
  .signal--falling { color: var(--color-text-muted); }
  .doubt { display: block; font-size: 0.72rem; color: var(--tone-higher); cursor: help; }
  .signal--climbing { color: var(--color-text); font-weight: 600; }

  .watch { display: flex; gap: 0.4rem; align-items: center; }
  .watch__all { display: inline-flex; gap: 0.35rem; align-items: center; cursor: pointer; }
  .watch__drop input { width: 3.6rem; font: inherit; font-size: 0.8rem; }
  .detail td { background: var(--color-bg); padding: 0.75rem 1rem 1rem; white-space: normal; }
  /* On a phone the table is wider than the screen: keep the chart screen-wide
     and in view while the table scrolls sideways. */
  .detail__inner { position: sticky; left: 1rem; max-width: calc(100vw - 4.25rem); }

  .explain { margin-top: 1rem; font-size: 0.85rem; color: var(--color-text-muted); }
  .explain summary { cursor: pointer; }
  .explain ul { padding-left: 1.1rem; }

  .sr-only {
    position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; border: 0;
    overflow: hidden; clip-path: inset(50%); white-space: nowrap;
  }

</style>
