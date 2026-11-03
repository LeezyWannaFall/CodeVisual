export class APIError extends Error {
 constructor(public status: number, public code: string, message: string) { super(message) }
}
export async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
 const res = await fetch(`/api/v1${path}`, { signal })
 if (!res.ok) {
  const body = await res.json().catch(() => null)
  throw new APIError(res.status, body?.error?.code ?? 'request_failed', body?.error?.message ?? `Request failed (${res.status})`)
 }
 return res.json() as Promise<T>
}
