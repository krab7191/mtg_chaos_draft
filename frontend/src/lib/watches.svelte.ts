// Watched product ids, shared by every island on a page (e.g. Cheap Packs'
// store listings and cheapest packs), so ticking a pack in one updates the
// other. The server is the source of truth; pages seed this from what they load.

import { SvelteSet } from 'svelte/reactivity';

export const watched = new SvelteSet<number>();

export function seedWatched(ids: Iterable<number>) {
  for (const id of ids) watched.add(id);
}

export function markWatched(id: number, on: boolean) {
  if (on) watched.add(id);
  else watched.delete(id);
}
