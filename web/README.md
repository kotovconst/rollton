# web/

Telegram Mini App for the `rolltonchatbot` (main bot). React 18 + TypeScript + Vite 7, Tailwind v4, Zustand + TanStack Query, `@telegram-apps/telegram-ui`.

## Prerequisites

- Node 20+
- The rolltonchatbot Go backend running locally (`make -C ../bot run BOT=rolltonchatbot`) for API calls to succeed. The stub will render but show errors until the backend implements the endpoints listed in the spec.

## First-time setup

```bash
cp .env.example .env.local           # adjust as needed; .env.local is gitignored
npm install
```

## Scripts

```bash
npm run dev          # vite dev server (default http://localhost:5173)
npm run build        # production build into dist/
npm run preview      # serve the prod bundle locally
npm run test         # vitest one-shot (CI mode)
npm run test:watch   # vitest watch mode
npm run lint         # eslint
npm run format       # prettier --write .
npm run typecheck    # tsc --noEmit
```

## Telegram-flavored dev

Mini apps must be served over HTTPS to load inside Telegram. For local dev:

1. Run `npm run dev` (Vite serves http://localhost:5173).
2. Expose it via a tunnel: `cloudflared tunnel --url http://localhost:5173`.
3. Tell @BotFather to set the mini app URL to the HTTPS tunnel URL (`/setmenubutton` → web app URL).
4. Open the bot in Telegram and tap the menu button.

For pure component dev, just open `http://localhost:5173/` — `<Layout>` renders `<OutsideTelegramNotice>` and stops further routing.

## Tech notes

- Telegram glue is in `src/lib/telegram.ts` as a thin facade over `window.Telegram.WebApp`. `index.html` loads `telegram-web-app.js` synchronously so the global is available before React mounts.
- Auth header: `Authorization: tma <initData>` — the rolltonchatbot Go backend validates HMAC.
- Server state in TanStack Query; client state in Zustand. Don't put server cache in Zustand.
- Tailwind v4 uses CSS-first config; there is no `tailwind.config.ts`. Customize via `src/styles/index.css` `@theme` block when needed.
- Tests use happy-dom (not jsdom) — happy-dom is faster and avoids the css-calc ESM clash on Node 20.18.
- Pinned to Vite 7 (not 8) because Vite 8 + rolldown require Node ≥ 20.19; the project's current Node baseline is 20.18.
