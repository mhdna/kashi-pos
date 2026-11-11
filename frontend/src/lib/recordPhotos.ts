import { ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// Record photos (expense, loan, transfer) come from the till's encrypted copy
// first and are checked with central in the background (record_photos.go).
// When central has something else now -- a photo turned, replaced, added or
// removed -- the till sends "record_photos_changed" with the record's ref:
// the expense or loan clientRef, the sent transfer's clientRef, or
// "transfer-<id>" for a received one (transferPhotoRef).

export const recordPhotosChangedEvent = "record_photos_changed";

// The ref the till names a transfer's photo by.
export function transferPhotoRef(clientRef: string, transferID: number): string {
  return clientRef ? clientRef : `transfer-${transferID}`;
}

// Per-record photo versions for a page: bumping a record's version changes
// its thumbnail's cache key, so RecordPhoto fetches it again instead of
// showing what it fetched before.
export function usePhotoVersions() {
  const versions = ref<Record<string, number>>({});
  function bump(ref: string) {
    if (!ref) return;
    versions.value = { ...versions.value, [ref]: (versions.value[ref] ?? 0) + 1 };
  }
  function cacheKey(prefix: string, ref: string): string {
    return `${prefix}-${ref}-${versions.value[ref] ?? 0}`;
  }
  return { versions, bump, cacheKey };
}

// Calls onChange with each changed record's ref; returns the unsubscribe.
export function onRecordPhotosChanged(onChange: (ref: string) => void): () => void {
  return EventsOn(recordPhotosChangedEvent, (ref: string) => {
    if (typeof ref === "string" && ref) onChange(ref);
  });
}
