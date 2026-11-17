import { toast } from './toast.svelte';

/** Plain-language explanation for an HTTP status, for when the server sent nothing useful. */
export function describeStatus(status: number): string {
  switch (status) {
    case 400: return 'the request was rejected';
    case 401: return 'your session expired — sign in again';
    case 403: return "you don't have permission to do that";
    case 404: return "it couldn't be found";
    case 409: return 'it conflicts with existing data';
    case 429: return 'too many requests — wait a moment and retry';
    case 502:
    case 503:
    case 504: return 'the server or a service it relies on is unreachable — try again shortly';
    default:  return status >= 500 ? `the server hit an error (${status})` : `the request failed (${status})`;
  }
}

/**
 * The best human-readable reason a response failed: the server's own short
 * plain-text message when it sent one, otherwise a description of the status.
 */
export async function errorReason(res: Response): Promise<string> {
  let body = '';
  try { body = (await res.text()).trim(); } catch { /* no readable body */ }
  // Skip HTML error pages (e.g. from the proxy) and anything too long for a toast.
  if (body && !body.startsWith('<') && body.length <= 200) return body;
  return describeStatus(res.status);
}

export const NETWORK_ERROR = "can't reach the server — check your connection";

/** Show "<failure>: <reason>" as an error toast. */
export async function toastResponseError(failure: string, res: Response): Promise<void> {
  toast.error(`${failure}: ${await errorReason(res)}`);
}

export function toastNetworkError(failure: string): void {
  toast.error(`${failure}: ${NETWORK_ERROR}`);
}

/**
 * GET JSON. On failure shows an error toast and returns null, so callers only
 * handle the happy path. Aborted requests (superseded searches) fail silently.
 */
export async function getJSON<T>(url: string, failure: string, init?: RequestInit): Promise<T | null> {
  try {
    const res = await fetch(url, init);
    if (!res.ok) {
      await toastResponseError(failure, res);
      return null;
    }
    return (await res.json()) as T;
  } catch (e) {
    if ((e as Error)?.name !== 'AbortError') toastNetworkError(failure);
    return null;
  }
}
