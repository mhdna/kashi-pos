# Agent notes for kashi-pos

## MDI icons must be registered in icons.ts

This project does not use the global MDI webfont. Icon strings like
`mdi-cash-plus` only render if they're imported from `@mdi/js` and added to
the `iconPaths` map in `frontend/src/lib/icons.ts`. An unregistered icon
name silently renders as raw text (with a console warning), not an icon.

Whenever you add a `v-icon` / `icon=` / `prepend-icon=` (etc.) with a new
`mdi-*` name anywhere in `frontend/src`, add the matching `mdi*` import and
`"mdi-*": mdi*` map entry in `frontend/src/lib/icons.ts` in the same change.
