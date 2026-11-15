// The unknown-items QR code of each picture session, kept for as long as the
// app runs. The QR strip is rebuilt whenever the sale screen comes back (a
// page switch, a hold restored); with this it shows the same code at once
// instead of a spinner while central is asked again.

export interface CachedQr {
  uploadUrl: string;
  dataUrl: string;
}

const cache = new Map<string, CachedQr>();

export function cachedQr(clientRef: string): CachedQr | undefined {
  return clientRef ? cache.get(clientRef) : undefined;
}

export function rememberQr(clientRef: string, qr: CachedQr): void {
  if (clientRef && qr.uploadUrl && qr.dataUrl) cache.set(clientRef, qr);
}

export function clearQrCache(): void {
  cache.clear();
}

// A new session id, made on the till before central is asked, so the page has
// it even if the screen goes away while that first call is still on its way.
export function newClientRef(): string {
  const c = globalThis.crypto as Crypto | undefined;
  if (c && typeof c.randomUUID === "function") return c.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
}
