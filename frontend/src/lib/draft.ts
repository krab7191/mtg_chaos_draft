/** A recorded pick: one draft slot. Small packs fill a slot with 2–3 packs. */
export interface PickLike {
  packId: number | null;
  setName: string;
  productType: string;
  /** Cost of the whole slot (unit price × packs in the slot). */
  marketPrice: number | null;
  /** Price of one physical pack, when known. */
  unitPrice?: number | null;
  /** Physical packs in the slot (defaults to 1). */
  packCount?: number;
  /** True while the live price is still being fetched. */
  pricing?: boolean;
}

/** All picks of the same pack, combined into one line. */
export interface PickGroup {
  key: string;
  setName: string;
  productType: string;
  /** Draft slots this pack fills. */
  slots: number;
  /** Physical packs to pull from the collection. */
  packs: number;
  /** How many of `each` make up `total` (packs when the unit price is known, else slots). */
  qty: number;
  /** Price of one item in `qty`, or null when unknown or not uniform. */
  each: number | null;
  /** Sum of priced picks, or null when none are priced. */
  total: number | null;
  /** Picks in this group with no price. */
  unpriced: number;
  pricing: boolean;
}

const sameValue = (values: (number | null | undefined)[]): number | null => {
  const first = values[0];
  if (first == null) return null;
  return values.every(v => v != null && Math.abs(v - first) < 0.005) ? first : null;
};

/** Combine picks of the same pack into one line each, in first-picked order. */
export function groupPicks(picks: PickLike[]): PickGroup[] {
  const groups = new Map<string, PickLike[]>();
  for (const p of picks) {
    const key = p.packId != null ? `id:${p.packId}` : `name:${p.setName}|${p.productType}`;
    const list = groups.get(key);
    if (list) list.push(p); else groups.set(key, [p]);
  }

  return Array.from(groups, ([key, list]) => {
    const slots = list.length;
    const packs = list.reduce((n, p) => n + (p.packCount ?? 1), 0);
    const priced = list.filter(p => p.marketPrice != null);
    const total = priced.length ? priced.reduce((s, p) => s + (p.marketPrice as number), 0) : null;

    // Prefer "packs × unit price"; fall back to "slots × slot price".
    const unit = sameValue(list.map(p => p.unitPrice));
    const slotPrice = sameValue(list.map(p => p.marketPrice));
    const [qty, each] = unit != null ? [packs, unit] : [slots, slotPrice];

    return {
      key,
      setName: list[0].setName,
      productType: list[0].productType,
      slots,
      packs,
      qty,
      each,
      total,
      unpriced: slots - priced.length,
      pricing: list.some(p => p.pricing),
    };
  });
}
