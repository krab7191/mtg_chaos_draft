import { toastNetworkError, toastResponseError } from './api';

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
      await toastResponseError(failure, res);
      return null;
    }
    if (res.status === 204) return true;
    try {
      return (await res.json()) as T;
    } catch {
      return true; // succeeded, but with no JSON body
    }
  } catch {
    toastNetworkError(failure);
    return null;
  }
}
