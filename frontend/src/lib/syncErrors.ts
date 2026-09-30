// Central-sync failures surface as the full Go error chain (see
// centralClient.responseError in sync_client.go), which is exactly what an
// admin debugging in this file's git history wants but useless to a till
// operator staring at a dialog: e.g. "compute deactivation candidates:
// reconcile products: reconcile products [0:500]: POST
// /branch/sync/reconcile failed (status 401): unknown branch".
//
// friendlySyncErrorReason maps the known, recurring cases to one short
// sentence a non-technical operator can act on. It never invents a reason
// for an error it doesn't recognize -- returning null there, so the caller
// falls back to showing the raw chain rather than a misleading guess.
export function friendlySyncErrorReason (raw: string): string | null {
  const msg = raw || ''

  if (/unknown branch/i.test(msg)) {
    return "kashi doesn't recognize this till's branch code. Check Settings → Central sync → Branch code."
  }
  if (/invalid branch key/i.test(msg)) {
    return "This till's API key doesn't match what kashi has on file. Check Settings → Central sync → Central API key."
  }
  if (/branch is disabled/i.test(msg)) {
    return 'This branch has been disabled in kashi. Ask an admin to reactivate it.'
  }
  if (/branch code and key headers are required/i.test(msg)) {
    return 'Central sync is not fully set up — enter both the branch code and the API key in Settings.'
  }
  if (/status 405/.test(msg)) {
    return "kashi rejected that request. Check the Central API URL in Settings includes \"/api\" (e.g. https://cloud.example.com/api)."
  }
  if (/status 401/.test(msg)) {
    return "kashi didn't accept this till's credentials. Check the branch code and API key in Settings → Central sync."
  }
  if (/status 404/.test(msg)) {
    return "kashi doesn't have this feature yet — it may need to be updated on the server. Contact an admin."
  }
  if (/status 5\d\d/.test(msg)) {
    return 'kashi ran into a problem on its end. Try again shortly, or contact an admin if it keeps happening.'
  }
  if (/no such host|dial tcp|connection refused/i.test(msg)) {
    return 'Could not reach kashi at this server address. Check the Central API URL in Settings and this till\'s internet connection.'
  }
  if (/context deadline exceeded|timeout/i.test(msg)) {
    return 'kashi took too long to respond. Check this till\'s internet connection and try again.'
  }
  if (/x509|certificate/i.test(msg)) {
    return "Couldn't make a secure connection to kashi (certificate problem). Contact an admin."
  }
  if (/central sync is not configured|central sync is not fully configured/i.test(msg)) {
    // Already a plain sentence -- nothing to translate, just pass it through
    // as "recognized" so the caller doesn't also show the (identical) raw
    // chain underneath.
    return msg
  }

  return null
}
