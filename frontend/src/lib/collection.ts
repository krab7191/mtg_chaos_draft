export interface Pack {
  id: number;
  setName: string;
  productType: string;
  marketPrice: number | null;
  quantity: number;
}

export function computeSortedSets(
  allPacks: Pack[],
  key: 'name' | 'price' | 'qty',
  dir: 'asc' | 'desc',
): [string, Pack[]][] {
  if (key === 'name') {
    const setMap = new Map<string, Pack[]>();
    for (const p of allPacks) {
      if (!setMap.has(p.setName)) setMap.set(p.setName, []);
      setMap.get(p.setName)!.push(p);
    }
    const sets = Array.from(setMap.entries());
    sets.sort(([a], [b]) => dir === 'asc' ? a.localeCompare(b) : b.localeCompare(a));
    // Non-empty packs first within each set
    sets.forEach(([, ps]) =>
      ps.sort((a, b) => (a.quantity === 0 ? 1 : 0) - (b.quantity === 0 ? 1 : 0))
    );
    return sets;
  }

  // Price / qty: each pack is its own card for true global ordering
  const fn = key === 'price'
    ? (p: Pack) => p.marketPrice ?? 0
    : (p: Pack) => p.quantity;

  return [...allPacks]
    .sort((a, b) => {
      const diff = fn(a) - fn(b);
      return dir === 'asc' ? diff : -diff;
    })
    .map(p => [String(p.id), [p]]);
}
