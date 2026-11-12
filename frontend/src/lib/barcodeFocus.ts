// When the sale screen may pull the focus back to the barcode field on its
// own. Searching in place (results over the calculator, keyboard along the
// bottom) owns the focus until it ends: the cashier is typing a code there.
// It is decided apart from the shared "dialogOpen" flag, which every dialog,
// menu and number pad on the screen writes; one closing set it false mid-search
// and the barcode field took the focus away from the search.

export interface FocusState {
  /** In-place product search is open. */
  searchingInPlace: boolean;
  /** The shared flag dialogs and menus set while they are open. */
  dialogOpen: boolean;
  /** A dialog, menu or overlay is in the DOM. */
  overlayOpen: boolean;
}

/** Something on screen owns the focus, so the barcode field must not take it. */
export function barcodeFocusBlocked(s: FocusState): boolean {
  return s.searchingInPlace || s.dialogOpen || s.overlayOpen;
}
