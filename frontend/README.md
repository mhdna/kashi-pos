# kashi-pos frontend

Vue 3 + Vite, run inside Wails. The UI uses our own small component kit in
`src/ui` (`k-btn`, `k-card`, `k-text-field`, ...): see
[`docs/dev/ui-components.md`](../docs/dev/ui-components.md).

```sh
pnpm install
pnpm dev            # or `make dev` at the repo root, to run the Wails app
pnpm test           # vitest
pnpm build          # icon check, type-check, production build
```
