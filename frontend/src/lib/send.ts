import { toast } from './toast.svelte';

/**
 * Send JSON to the API. Returns the parsed response (or true for an empty
 * one), or null after showing the server's message in an error toast.
 */
export async function send<T = unknown>(url: string, method: string, body: unknown, failure: string): Promise<T | true | null> {
  try {
    const res = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const msg = (await res.text().catch(() => '')).trim();
      toast.show(`${failure}: ${msg || res.status}`, 'error');
      return null;
    }
    return res.status === 204 ? true : ((await res.json()) as T);
  } catch {
    toast.show(failure, 'error');
    return null;
  }
}
