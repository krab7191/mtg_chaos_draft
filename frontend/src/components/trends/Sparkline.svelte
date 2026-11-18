<script lang="ts">
  // Tiny price line for a table row: weekly prices since release, with the
  // latest price marked. Purely visual; the row's cells carry the numbers.
  let { values, width = 96, height = 28 }: { values: number[]; width?: number; height?: number } = $props();

  const PAD = 3;
  const geometry = $derived.by(() => {
    if (values.length < 2) return null;
    const lo = Math.min(...values), hi = Math.max(...values);
    const span = hi - lo || 1;
    const x = (i: number) => PAD + (i / (values.length - 1)) * (width - PAD * 2);
    const y = (v: number) => PAD + (1 - (v - lo) / span) * (height - PAD * 2);
    const path = values.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join('');
    const last = values.length - 1;
    return { path, endX: x(last), endY: y(values[last]), lowY: y(lo) };
  });
</script>

{#if geometry}
  <svg class="spark" {width} {height} viewBox="0 0 {width} {height}" aria-hidden="true">
    <line class="spark__low" x1={PAD} x2={width - PAD} y1={geometry.lowY} y2={geometry.lowY} />
    <path class="spark__line" d={geometry.path} />
    <circle class="spark__end" cx={geometry.endX} cy={geometry.endY} r="2.5" />
  </svg>
{:else}
  <span class="spark spark--empty">no history yet</span>
{/if}

<style>
  .spark { display: block; overflow: visible; }
  .spark__line { fill: none; stroke: var(--chart-line); stroke-width: 1.5; stroke-linejoin: round; stroke-linecap: round; }
  .spark__low  { stroke: var(--color-border); stroke-width: 1; stroke-dasharray: 2 2; }
  .spark__end  { fill: var(--chart-line); stroke: var(--color-surface); stroke-width: 1.5; }
  .spark--empty { font-size: 0.72rem; color: var(--color-text-muted); }
</style>
