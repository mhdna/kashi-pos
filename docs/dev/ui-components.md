# UI components (`frontend/src/ui`)

The till does **not** use a component library. Vuetify was removed on the
`no-vuetify` branch because it made the app slow on older machines: its
JavaScript and CSS were the biggest part of the bundle, and every button, field
and list item was a deep tree of reactive components.

| | Before (Vuetify 4) | After |
|---|---|---|
| JS in `dist/assets` | 1.4 MB | 1.06 MB |
| CSS in `dist/assets` | 362 KB | 80 KB |
| Main chunk, gzipped | 143 KB | 94 KB |

(`pnpm build` in `frontend/`; sizes from the build output.)

## How it is put together

- **Components** are `K*.vue` files in `frontend/src/ui`, registered globally by
  `src/ui/index.ts` and used as `<k-btn>`, `<k-card>`, `<k-text-field>`... The
  names and props follow what Vuetify had, but **only the props, variants and
  slots the app actually uses exist**. Adding a prop is fine; adding a component
  the app doesn't use is not.
- **Types**: `src/ui/global.d.ts` declares the components, so templates are
  type-checked by `vue-tsc`.
- **No transitions, no ripples.** Only the spinners (`k-progress-circular`,
  `k-progress-linear`) animate.
- **Closed overlays cost nothing.** Dialogs, menus, tooltips and snackbars are
  rendered only while open, and teleported to `<body>`. Without a `v-model`
  (`<k-menu>` / `<k-dialog>` with an `#activator`) they keep their own open
  state.

## CSS

- `src/ui/ui.css` (components) and `src/ui/utilities.css` (the `d-flex`,
  `pa-4`, `text-center`... classes the app uses) live in **cascade layers**
  (`k-core`, `k-components`, `k-utilities`). Any unlayered CSS, such as a page's
  `<style scoped>`, therefore always beats them, the same as it did with
  Vuetify. Don't remove the layers.
- Theme colours are CSS custom properties set on `<html>` by `src/ui/theme.ts`
  as **`r,g,b` triplets**: `rgb(var(--k-theme-primary))`,
  `rgba(var(--k-theme-on-surface), .12)`. Also `--k-border-color`,
  `--k-border-opacity`, `--k-medium-emphasis-opacity`... The theme class is
  `k-theme--light` / `k-theme--dark` on `<html>`.
- `useTheme()` (from `@/ui`) returns `{ name, change(name) }`.
- A `color` prop takes a theme name (`primary`, `error`...), a Material name
  (`blue-grey-darken-1`, from `src/ui/palette.ts`) or any CSS colour. See
  `resolveColor` / `onColor` in `src/ui/colors.ts`.
- The classes inside components keep Vuetify's shape where page CSS reaches in
  with `:deep()`: `.k-field`, `.k-input`, `.k-list-item__content`,
  `.k-overlay__content`, `.k-snackbar__wrapper`, `.k-table__wrapper`,
  `.k-card-title`... Utility classes that Vuetify 4 never defined (`text-h6`,
  `text-body-2`, ...) are no-ops here too.

## Things that behave in a way you might not expect

- `<k-main>` pads itself by `--k-layout-left`, which `<k-navigation-drawer>`
  sets to its measured width while mounted.
- `<k-data-table-virtual>` / `<k-data-table>` are one component. Rows are
  windowed only when `item-height` is given. Clicking a header sorts
  client-side (ascending, descending, off) unless the header has
  `sortable: false`. `@click:row` gets `(event, { item, index })`.
- `<k-virtual-scroll>` and the table expose `scrollToIndex(i)`; `$el` is the
  scroll container.
- `<k-form>`'s `validate()` resolves `{ valid }`. Fields register their `rules`
  with the nearest form.
- `<k-text-field>`'s `$el` contains the `<input>` (several places query it).
  `class`/`style` go on the outer element, every other attribute and listener
  (`@keydown`, `min`, `inputmode`...) on the `<input>`.
- Escape closes only the topmost overlay (`src/ui/overlay.ts`).
- A snackbar's `timeout` defaults to 5000 ms; `-1` keeps it up.

## Tests

`src/ui/*.test.ts` cover the logic that isn't just markup: colours, the overlay
stack, floating-box placement, select items, form rules, the virtual window and
sorting. Visual checks: render the component in a throwaway page and compare
light and dark.
