import { describe, it, expect } from 'vitest';
import { render, fireEvent, act } from '../../tests/svelte';
import Sparkline from './Sparkline.svelte';
import PriceChart from './PriceChart.svelte';

describe('Sparkline', () => {
  it('draws a line, the low, and the latest point', () => {
    const { container } = render(Sparkline, { props: { values: [10, 8, 6, 7] } });
    expect(container.querySelector('.spark__line')?.getAttribute('d')).toMatch(/^M.*L.*L.*L/);
    expect(container.querySelector('.spark__low')).not.toBeNull();
    expect(container.querySelector('.spark__end')).not.toBeNull();
  });

  it('handles flat and too-short series', () => {
    const flat = render(Sparkline, { props: { values: [5, 5, 5] } });
    expect(flat.container.querySelector('.spark__line')).not.toBeNull();
    const short = render(Sparkline, { props: { values: [5] } });
    expect(short.container.textContent).toContain('no history yet');
  });
});

const day = (i: number) => new Date(Date.UTC(2024, 0, 1 + i)).toISOString().slice(0, 10);
const points = (n: number, f: (i: number) => number) => Array.from({ length: n }, (_, i) => ({ day: day(i), price: f(i) }));

describe('PriceChart', () => {
  function svgRect(container: HTMLElement) {
    const svg = container.querySelector('svg')!;
    svg.getBoundingClientRect = () => ({ left: 0, top: 0, width: 720, height: 220, right: 720, bottom: 220, x: 0, y: 0, toJSON: () => ({}) });
    return svg;
  }

  it('shows axes, the release marker, the low line, and a tooltip on hover', async () => {
    const { container } = render(PriceChart, {
      props: { points: points(800, i => 10 - i / 200), released: day(100), low: 6.5, perPackDivisor: 36, title: 'Box price' },
    });
    expect(container.querySelector('.chart__title')?.textContent).toBe('Box price');
    // Opens on the past year: release (day 100 of 800) is out of view until "All".
    expect(container.textContent).not.toContain('release');
    await act(() => fireEvent.click(Array.from(container.querySelectorAll('.chart__ranges button')).find(b => b.textContent === 'All')!));
    expect(container.querySelectorAll('text.axis').length).toBeGreaterThan(3);
    expect(container.textContent).toContain('release');
    // Before release is shaded as preorder; ~2 years of data gets quarterly labels.
    expect(container.querySelector('rect.preorder')).not.toBeNull();
    expect(container.textContent).toContain('preorder');
    const labels = Array.from(container.querySelectorAll('text.axis')).map(t => t.textContent ?? '').filter(t => !t.startsWith('$'));
    expect(labels.length).toBeGreaterThanOrEqual(6);
    expect(container.textContent).toContain('low $6.50');

    svgRect(container);
    await act(() => fireEvent.pointerMove(container.querySelector('.hit')!, { clientX: 400, clientY: 100 }));
    const tip = container.querySelector('.tip')!;
    expect(tip.textContent).toMatch(/\$\d+\.\d\d/);
    expect(tip.textContent).toContain('/ pack');
    await act(() => fireEvent.pointerLeave(container.querySelector('.hit')!));
    expect(container.querySelector('.tip')).toBeNull();
  });

  it('reads prices with the arrow keys', async () => {
    const { container } = render(PriceChart, { props: { points: points(60, i => 5 + i / 10), title: 'Pack' } });
    const svg = container.querySelector('svg')!;
    await act(() => fireEvent.keyDown(svg, { key: 'ArrowLeft' }));
    expect(container.querySelector('.tip')?.textContent).toContain('$10.80'); // last-but-one day
    await act(() => fireEvent.keyDown(svg, { key: 'ArrowRight', shiftKey: true }));
    expect(container.querySelector('.tip')?.textContent).toContain('$10.90');
    await act(() => fireEvent.keyDown(svg, { key: 'Enter' }));
    await act(() => fireEvent.blur(svg));
    expect(container.querySelector('.tip')).toBeNull();
  });

  it('uses year labels for long histories', async () => {
    const long = render(PriceChart, { props: { points: points(1500, () => 4), title: 'Pack' } });
    await act(() => fireEvent.click(Array.from(long.container.querySelectorAll('.chart__ranges button')).find(b => b.textContent === 'All')!));
    const labels = Array.from(long.container.querySelectorAll('text.axis')).map(t => t.textContent ?? '').filter(t => !t.startsWith('$'));
    expect(labels).toEqual(['2024', '2025', '2026', '2027', '2028']); // Jan 2024 → Feb 2028
  });

  it('uses month labels for short histories and copes with too little data', () => {
    const short = render(PriceChart, { props: { points: points(120, () => 4), title: 'Pack' } });
    expect(Array.from(short.container.querySelectorAll('text.axis')).some(t => /[A-Za-z]/.test(t.textContent ?? ''))).toBe(true);
    const none = render(PriceChart, { props: { points: points(1, () => 4), title: 'Pack' } });
    expect(none.container.textContent).toContain('Not enough price history');
  });

  it('picks sensible price ticks for a flat line and for a wide range', () => {
    const flat = render(PriceChart, { props: { points: points(30, () => 120), title: 'Flat' } });
    const flatTicks = Array.from(flat.container.querySelectorAll('text.axis')).map(t => t.textContent);
    expect(flatTicks.some(t => t?.startsWith('$1'))).toBe(true);

    const wide = render(PriceChart, { props: { points: points(30, i => 20 + i * 40), title: 'Wide' } });
    const wideTicks = Array.from(wide.container.querySelectorAll('text.axis')).map(t => t.textContent ?? '').filter(t => t.startsWith('$'));
    expect(wideTicks.length).toBeGreaterThanOrEqual(3);
    expect(wideTicks.every(t => /^\$\d+$/.test(t))).toBe(true); // whole dollars above $10
  });

  it('defaults to the past year with quarterly labels, and offers only useful ranges', async () => {
    const twoYears = render(PriceChart, { props: { points: points(730, i => 10 + (i % 50) / 10), low: 3, title: 'Box' } });
    const buttons = () => Array.from(twoYears.container.querySelectorAll('.chart__ranges button')).map(b => b.textContent);
    expect(buttons()).toEqual(['3M', '6M', '1Y', 'All']); // 2 years of data: 3Y would equal All
    expect(twoYears.container.querySelector('.chart__ranges .on')?.textContent).toBe('1Y');
    const labels = () => Array.from(twoYears.container.querySelectorAll('text.axis')).map(t => t.textContent ?? '').filter(t => !t.startsWith('$'));
    expect(labels().length).toBeGreaterThanOrEqual(3);
    expect(labels().length).toBeLessThanOrEqual(5); // quarters in a year
    // A low far outside the visible prices isn't drawn off-chart.
    expect(twoYears.container.textContent).not.toContain('low $3.00');

    await act(() => fireEvent.click(Array.from(twoYears.container.querySelectorAll('.chart__ranges button')).find(b => b.textContent === 'All')!));
    expect(twoYears.container.querySelector('.chart__ranges .on')?.textContent).toBe('All');
    expect(labels().length).toBeGreaterThan(5);
    // Short ranges zoom in: 3 months gets monthly labels.
    await act(() => fireEvent.click(Array.from(twoYears.container.querySelectorAll('.chart__ranges button')).find(b => b.textContent === '3M')!));
    expect(labels().length).toBeGreaterThanOrEqual(2);
    expect(labels().length).toBeLessThanOrEqual(4);

    // Under a year: opens on everything, with 3M and 6M to zoom in.
    const young = render(PriceChart, { props: { points: points(200, () => 4), title: 'New' } });
    expect(Array.from(young.container.querySelectorAll('.chart__ranges button')).map(b => b.textContent)).toEqual(['3M', '6M', 'All']);
    expect(young.container.querySelector('.chart__ranges .on')?.textContent).toBe('All');
    // Under 3 months: nothing to choose.
    const brandNew = render(PriceChart, { props: { points: points(60, () => 4), title: 'Brand new' } });
    expect(brandNew.container.querySelector('.chart__ranges')).toBeNull();
  });

  it('keeps the tooltip inside the chart at either edge', async () => {
    const { container } = render(PriceChart, { props: { points: points(60, i => 5 + i / 10), title: 'Pack' } });
    const svg = container.querySelector('svg')!;
    await act(() => fireEvent.keyDown(svg, { key: 'ArrowLeft' })); // last-but-one day: right edge
    expect(container.querySelector('.tip')?.classList.contains('tip--right')).toBe(true);
    for (let i = 0; i < 60; i++) await act(() => fireEvent.keyDown(svg, { key: 'ArrowLeft' }));
    expect(container.querySelector('.tip')?.classList.contains('tip--left')).toBe(true);
  });
});
