# hyperx

> pure-go x11 toolkit, shared by the `hyper*` suite

No cgo. No third-party X11 deps. Speaks the X11 wire protocol directly.

Packages:

- `xproto` — minimal X11 core protocol client
- `drw` — drawing primitives (rects, text, color schemes)
- `keysym` — keysym constants and rune mapping
- `fontcfg` — fontconfig-style font discovery and matching
- `xinerama` — multi-monitor query

Used by [hypermenu](https://github.com/alexisbchz/hypermenu) and
[hyperlogin](https://github.com/alexisbchz/hyperlogin).

## License

[MIT](./LICENSE)
