<script lang="ts">
  // Daily price chart for one product: the price line, a dashed line at the
  // low since release, a marker at release, and a crosshair + tooltip that
  // snaps to the nearest day.
  import type { PricePoint } from '../../lib/trends';
  import { money } from '../../lib/trends';

  let { points, released = null, low = null, perPackDivisor = 1, title }: {
    points: PricePoint[];
    released?: string | null;
    low?: number | null;
    /** Show a box's price per pack too when > 1. */
    perPackDivisor?: number;
    title: string;
  } = $props();

  const W = 720, H = 220;
  const M = { top: 12, right: 12, bottom: 24, left: 52 };
  const iw = W - M.left - M.right, ih = H - M.top - M.bottom;

  const DAY = 86_400_000;
  const all = $derived(points.map(p => ({ t: new Date(p.day).getTime(), v: p.price })));
  const spanDays = $derived(all.length > 1 ? (all[all.length - 1].t - all[0].t) / DAY : 0);

  // Visible range: the past year by default (everything, for younger
  // products), with shorter and longer views a click away. Only ranges
  // shorter than the history are offered, plus "All".
  type Range = '3m' | '6m' | '1y' | '3y' | 'all';
  const RANGES: { id: Range; label: string; days: number }[] = [
    { id: '3m', label: '3M', days: 91 },
    { id: '6m', label: '6M', days: 182 },
    { id: '1y', label: '1Y', days: 365 },
    { id: '3y', label: '3Y', days: 1095 },
    { id: 'all', label: 'All', days: Infinity },
  ];
  let picked = $state<Range | null>(null);
  const ranges = $derived(RANGES.filter(r => r.days < spanDays || r.id === 'all'));
  const range = $derived<Range>(picked ?? (spanDays > 365 ? '1y' : 'all'));
  const data = $derived.by(() => {
    const days = RANGES.find(r => r.id === range)!.days;
    if (!all.length || !Number.isFinite(days)) return all;
    const cutoff = all[all.length - 1].t - days * DAY;
    return all.filter(p => p.t >= cutoff);
  });
  function pickRange(r: Range) { picked = r; hover = null; }

  const scale = $derived.by(() => {
    if (data.length < 2) return null;
    const t0 = data[0].t, t1 = data[data.length - 1].t;
    let lo = Math.min(...data.map(d => d.v)), hi = Math.max(...data.map(d => d.v));
    const pad = (hi - lo) * 0.08 || hi * 0.1 || 1;
    lo = Math.max(0, lo - pad); hi += pad;
    const x = (t: number) => M.left + ((t - t0) / (t1 - t0 || 1)) * iw;
    const y = (v: number) => M.top + (1 - (v - lo) / (hi - lo)) * ih;
    // ~4 round price ticks.
    const step = niceStep((hi - lo) / 4);
    const ticks: number[] = [];
    for (let v = Math.ceil(lo / step) * step; v <= hi; v += step) ticks.push(v);
    // Time ticks: monthly for short histories, quarterly up to ~3 years,
    // then yearly. Months start on calendar quarters (Jan, Apr, Jul, Oct).
    const spanDays = (t1 - t0) / 86_400_000;
    const months = spanDays <= 200 ? 1 : spanDays <= 1100 ? 3 : 12;
    const xticks: { t: number; label: string }[] = [];
    const d = new Date(t0);
    d.setUTCDate(1);
    d.setUTCMonth(Math.floor(d.getUTCMonth() / months) * months);
    for (; d.getTime() <= t1; d.setUTCMonth(d.getUTCMonth() + months)) {
      if (d.getTime() >= t0) {
        xticks.push({
          t: d.getTime(),
          label: months === 12
            ? String(d.getUTCFullYear())
            : d.toLocaleDateString(undefined, { month: 'short', year: '2-digit', timeZone: 'UTC' }),
        });
      }
    }
    const path = data.map((p, i) => `${i ? 'L' : 'M'}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`).join('');
    // Whole dollars once ticks are a dollar or more apart.
    const tickLabel = (v: number) => `$${step >= 1 ? v.toFixed(0) : v.toFixed(2)}`;
    return { x, y, ticks, xticks, path, t0, t1, tickLabel, lo, hi };
  });

  function niceStep(raw: number): number {
    const mag = 10 ** Math.floor(Math.log10(raw || 1));
    const n = raw / mag;
    return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10) * mag;
  }

  let hover = $state<number | null>(null);

  function onMove(e: PointerEvent) {
    if (!scale) return;
    const svg = (e.currentTarget as SVGElement).closest('svg')!;
    const rect = svg.getBoundingClientRect();
    const px = ((e.clientX - rect.left) / rect.width) * W;
    const t = scale.t0 + ((px - M.left) / iw) * (scale.t1 - scale.t0);
    // Nearest day by binary search.
    let lo = 0, hi = data.length - 1;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if (data[mid].t < t) lo = mid + 1; else hi = mid;
    }
    if (lo > 0 && Math.abs(data[lo - 1].t - t) < Math.abs(data[lo].t - t)) lo--;
    hover = lo;
  }

  function onKey(e: KeyboardEvent) {
    if (!data.length) return;
    const step = e.shiftKey ? 30 : 1;
    if (e.key === 'ArrowLeft') hover = Math.max(0, (hover ?? data.length - 1) - step);
    else if (e.key === 'ArrowRight') hover = Math.min(data.length - 1, (hover ?? 0) + step);
    else return;
    e.preventDefault();
  }

  const fmtDay = (t: number) => new Date(t).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric', timeZone: 'UTC' });
  const releasedT = $derived(released ? new Date(released).getTime() : null);
</script>

<figure class="chart">
  <figcaption class="chart__head">
    <span class="chart__title">{title}</span>
    {#if ranges.length > 1}
      <div class="chart__ranges" role="group" aria-label="Time range">
        {#each ranges as r (r.id)}
          <button class:on={range === r.id} aria-pressed={range === r.id} onclick={() => pickRange(r.id)}>{r.label}</button>
        {/each}
      </div>
    {/if}
  </figcaption>
  {#if scale}
    <!-- The chart is focusable so arrow keys can read prices day by day. -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <svg
      viewBox="0 0 {W} {H}"
      role="img"
      aria-label="{title}: daily price from {fmtDay(scale.t0)} to {fmtDay(scale.t1)}. Use arrow keys to read prices."
      tabindex="0"
      onkeydown={onKey}
      onblur={() => (hover = null)}
    >
      {#each scale.ticks as v (v)}
        <line class="grid" x1={M.left} x2={W - M.right} y1={scale.y(v)} y2={scale.y(v)} />
        <text class="axis" x={M.left - 6} y={scale.y(v)} text-anchor="end" dominant-baseline="middle">{scale.tickLabel(v)}</text>
      {/each}
      {#each scale.xticks as tk (tk.t)}
        <text class="axis" x={scale.x(tk.t)} y={H - 6} text-anchor="middle">{tk.label}</text>
      {/each}

      {#if releasedT != null && releasedT >= scale.t0 && releasedT <= scale.t1}
        <!-- Prices before release are preorders: shaded, and not used for the low. -->
        {#if releasedT > scale.t0}
          <rect class="preorder" x={M.left} y={M.top} width={scale.x(releasedT) - M.left} height={ih} />
          <text class="marker__label" x={M.left + 4} y={M.top + 10}>preorder</text>
        {/if}
        <line class="marker" x1={scale.x(releasedT)} x2={scale.x(releasedT)} y1={M.top} y2={H - M.bottom} />
        <text class="marker__label" x={scale.x(releasedT) + 4} y={M.top + 10}>release</text>
      {/if}
      {#if low != null && low >= scale.lo && low <= scale.hi}
        <line class="low" x1={M.left} x2={W - M.right} y1={scale.y(low)} y2={scale.y(low)} />
        <text class="marker__label" x={W - M.right} y={scale.y(low) - 4} text-anchor="end">low {money(low)}</text>
      {/if}

      <path class="line" d={scale.path} />

      {#if hover != null}
        {@const p = data[hover]}
        <line class="crosshair" x1={scale.x(p.t)} x2={scale.x(p.t)} y1={M.top} y2={H - M.bottom} />
        <circle class="dot" cx={scale.x(p.t)} cy={scale.y(p.v)} r="4" />
      {/if}

      <!-- Hit area bigger than the line: the whole plot. -->
      <rect
        class="hit" x={M.left} y={M.top} width={iw} height={ih}
        role="presentation"
        onpointermove={onMove}
        onpointerleave={() => (hover = null)}
      />
    </svg>
    {#if hover != null}
      {@const p = data[hover]}
      <div class="tip" style:left="{(scale.x(p.t) / W) * 100}%">
        <span class="tip__day">{fmtDay(p.t)}</span>
        <strong>{money(p.v)}</strong>
        {#if perPackDivisor > 1}<span class="tip__sub">{money(p.v / perPackDivisor)} / pack</span>{/if}
      </div>
    {/if}
  {:else}
    <p class="chart__empty">Not enough price history yet.</p>
  {/if}
</figure>

<style>
  .chart { position: relative; margin: 0; }
  .chart__head { display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem; }
  .chart__title { font-size: 0.8rem; color: var(--color-text-muted); }
  .chart__ranges { display: inline-flex; border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
  .chart__ranges button {
    background: none; border: none; color: var(--color-text-muted); font: inherit; font-size: 0.75rem;
    padding: 0.15rem 0.55rem; cursor: pointer;
  }
  .chart__ranges button + button { border-left: 1px solid var(--color-border); }
  .chart__ranges button.on { background: var(--color-surface); color: var(--color-text); font-weight: 600; }
  .chart__empty { font-size: 0.85rem; color: var(--color-text-muted); }
  svg { display: block; width: 100%; height: auto; outline: none; }
  svg:focus-visible { outline: 2px solid var(--color-accent); outline-offset: 2px; border-radius: var(--radius); }
  .grid { stroke: var(--color-border); stroke-width: 1; }
  .axis { fill: var(--color-text-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
  .line { fill: none; stroke: var(--chart-line); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
  .low { stroke: var(--color-text-muted); stroke-width: 1; stroke-dasharray: 4 3; }
  .marker { stroke: var(--color-text-muted); stroke-width: 1; stroke-dasharray: 2 3; }
  .preorder { fill: var(--color-text-muted); opacity: 0.07; }
  .marker__label { fill: var(--color-text-muted); font-size: 10px; }
  .crosshair { stroke: var(--color-text); stroke-width: 1; opacity: 0.5; }
  .dot { fill: var(--chart-line); stroke: var(--color-surface); stroke-width: 2; }
  .hit { fill: transparent; cursor: crosshair; }
  .tip {
    position: absolute;
    top: 1.5rem;
    transform: translateX(-50%);
    pointer-events: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.3rem 0.5rem;
    font-size: 0.78rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
  .tip__day, .tip__sub { color: var(--color-text-muted); font-size: 0.72rem; }
</style>
