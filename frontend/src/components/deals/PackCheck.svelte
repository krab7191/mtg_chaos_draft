<script lang="ts">
  // "Check a pack": search any single pack and see what its contents are
  // worth (pull rates x card prices), by slot and by card, against its price.
  import { getJSON } from '../../lib/api';
  import { money } from '../../lib/trends';
  import { chanceText, ratioText, type Breakdown } from '../../lib/deals';

  interface Product { mtgstocksId: number; setName: string; productType: string; marketPrice: number | null }

  let query = $state('');
  let results = $state<Product[]>([]);
  let searching = $state(false);
  let picked = $state<Product | null>(null);
  let result = $state<Breakdown | null>(null);
  let loading = $state(false);
  let timer: ReturnType<typeof setTimeout> | null = null;
  let seq = 0;

  function onInput(e: Event) {
    query = (e.currentTarget as HTMLInputElement).value;
    if (timer) clearTimeout(timer);
    const q = query.trim();
    if (q.length < 2) { results = []; searching = false; return; }
    searching = true;
    timer = setTimeout(() => search(q), 350);
  }

  async function search(q: string) {
    const mine = ++seq;
    const items = await getJSON<Product[]>(`/api/search?q=${encodeURIComponent(q)}`, 'Search failed');
    if (mine !== seq) return;
    results = (Array.isArray(items) ? items : []).slice(0, 8);
    searching = false;
  }

  async function check(p: Product) {
    seq++; // drop a search still in flight
    picked = p;
    results = [];
    query = `${p.setName} ${p.productType}`;
    result = null;
    loading = true;
    result = await getJSON<Breakdown>(`/api/packs/${p.mtgstocksId}/contents`, "Couldn't work out that pack's contents");
    loading = false;
  }
</script>

<section class="check" aria-labelledby="check-title">
  <h2 id="check-title" class="check__title">Check a pack</h2>
  <p class="check__sub">What the cards inside a pack are worth on average (MTGJSON pull rates × TCGplayer card prices), next to what it costs.</p>

  <div class="search">
    <input type="search" placeholder="e.g. Ravnica Allegiance collector" aria-label="Find a pack" value={query} oninput={onInput} />
    {#if searching}<span class="note">Searching…</span>{/if}
    {#if results.length > 0}
      <ul class="results">
        {#each results as p (p.mtgstocksId)}
          <li><button type="button" class="link" onclick={() => check(p)}>{p.setName} — {p.productType}</button>
            <span class="muted">{money(p.marketPrice)}</span></li>
        {/each}
      </ul>
    {/if}
  </div>

  {#if loading}
    <p class="note">Working it out…</p>
  {:else if result}
    {@const r = result}
    <div class="summary" class:premium={r.premium}>
      <p class="summary__line">
        <strong>{r.setName} {r.productType}</strong>: contents worth <strong>{money(r.value)}</strong> per pack
        {#if r.price}against a price of <strong>{money(r.price)}</strong>{#if r.noSales}{' '}(asking prices only: no recent sales){/if}{/if}.
      </p>
      {#if r.ratio}<p class="summary__verdict">{r.premium ? '⚠ ' : ''}{ratioText(r.ratio)}.</p>{/if}
      <p class="note">English card prices from {r.pricesDay}; foreign-language cards can sell for more or less. Bulk commons count at market price but are hard to sell.</p>
    </div>

    <div class="cols">
      <div>
        <h3 class="check__h3">By slot ({Math.round(r.cards)} cards)</h3>
        <table class="mini">
          <tbody>
            {#each r.slots as s (s.name)}
              <tr><td>{s.count % 1 === 0 ? s.count : s.count.toFixed(1)} × {s.name}</td><td class="num">{money(s.value)}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
      <div>
        <h3 class="check__h3">Cards that add the most</h3>
        <table class="mini">
          <tbody>
            {#each r.top as c (c.name + c.foil)}
              <tr>
                <td>{c.name}{c.foil ? ' (foil)' : ''}<span class="sub">{chanceText(c.chance)}</span></td>
                <td class="num">{money(c.price)}<span class="sub">adds {money(c.value)}</span></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {:else if picked}
    <p class="note">No breakdown for {picked.setName} {picked.productType}.</p>
  {/if}
</section>

<style>
  .check { margin-bottom: 2rem; }
  .check__title { font-size: 1.1rem; margin: 0 0 0.3rem; }
  .check__sub { color: var(--color-text-muted); font-size: 0.85rem; margin: 0 0 0.8rem; }
  .check__h3 { font-size: 0.85rem; margin: 0.8rem 0 0.3rem; }
  .note { font-size: 0.8rem; color: var(--color-text-muted); }
  .muted { color: var(--color-text-muted); font-size: 0.8rem; margin-left: 0.4rem; }

  .search { position: relative; max-width: 32rem; }
  .search input {
    width: 100%; padding: 0.4rem 0.6rem; border: 1px solid var(--color-border); border-radius: var(--radius);
    background: var(--color-surface); color: var(--color-text); font: inherit; font-size: 0.9rem;
  }
  .results { list-style: none; margin: 0.3rem 0 0; padding: 0.3rem 0.5rem; border: 1px solid var(--color-border); border-radius: var(--radius); }
  .results li { padding: 0.2rem 0; font-size: 0.85rem; }
  .link { background: none; border: none; padding: 0; font: inherit; color: var(--color-accent); cursor: pointer; text-align: left; }

  .summary { margin-top: 0.9rem; padding: 0.6rem 0.8rem; border: 1px solid var(--color-border); border-radius: var(--radius); }
  .summary.premium { border-color: var(--color-danger); }
  .summary__line { margin: 0 0 0.3rem; font-size: 0.9rem; }
  .summary__verdict { margin: 0 0 0.3rem; font-size: 0.9rem; }

  .cols { display: grid; grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr)); gap: 0 1.5rem; }
  .mini { width: 100%; border-collapse: collapse; font-size: 0.82rem; font-variant-numeric: tabular-nums; }
  .mini td { padding: 0.3rem 0.4rem; border-bottom: 1px solid var(--color-border); vertical-align: top; }
  .mini .num { text-align: right; white-space: nowrap; }
  .sub { display: block; font-size: 0.72rem; color: var(--color-text-muted); }
</style>
