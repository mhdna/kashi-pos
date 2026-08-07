# README

## About

This is the official Wails Vue template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

### Central and branch builds

The default build is a branch build. Branch builds show the mock Import page;
central builds also enable Inventory, use a red appbar, and default the branch
name to `المركزي`. Page selection is controlled at build time with
`VITE_KASHI_ENABLED_PAGES`.

Build a central package with a private key supplied only by the build
environment:

```sh
KASHI_CENTRAL_PRIVATE_KEY='your-private-key' make build-central
```

The private key is passed to the backend build and is not included in the
frontend bundle. Add future central-only catalog mutations behind
`requireCentralBuild()` in the backend.

## Embedded remote admin API

The POS can optionally expose the admin API from the same process that owns the SQLite database. This avoids running a separate database agent on the POS device. Set these environment variables before launching the app:

```sh
export KASHI_ADMIN_ENABLED=1
export KASHI_ADMIN_LISTEN=0.0.0.0:8787
export KASHI_ADMIN_TOKEN='choose-a-secret'
```

Use `127.0.0.1:8787` (the default) for local-only access. A token is required for non-loopback listeners. The existing TUI client in `tools/kashi-admin` can connect with `--server` and `--token`. Its reload action emits an app event that refreshes the running frontend.
