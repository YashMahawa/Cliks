# Cliks

Cliks is an ambient coworking prototype. It lets a remote team share the feeling of working in the same room without sharing what anyone is typing.

The system has three parts:

- `site`: a Vercel-hosted Next.js site for creating team codes.
- `server`: a Render-hosted API and WebSocket relay.
- `cli`: the `typ` command that joins rooms, captures local activity pulses, and plays remote ambience.

## Privacy model

The CLI never sends actual keys, key codes, text, mouse coordinates, window names, app names, clipboard data, screenshots, or microphone audio.

It sends only batched activity pulses:

```json
{
  "type": "activity_batch",
  "teamCode": "CLIK-842K",
  "events": [
    { "kind": "keyboard", "offsetMs": 0 },
    { "kind": "keyboard", "offsetMs": 84 },
    { "kind": "mouse", "button": "left", "offsetMs": 301 }
  ]
}
```

Events are batched every 500ms by default, while preserving the exact kind and interval offsets inside the batch.

## Local setup

```bash
cd cliks
npm install
cp server/.env.example server/.env
cp site/.env.example site/.env.local
npm run dev:server
```

In another terminal:

```bash
cd cliks
npm run dev:site
```

In another terminal:

```bash
cd cliks
npm --workspace @cliks/cli run dev -- join CLIK-LOCAL
npm --workspace @cliks/cli run dev
```

Without Supabase env vars, the server uses an in-memory store for local development.

## Deploy

Render should run `server`:

```bash
npm install
npm --workspace @cliks/server run build
npm --workspace @cliks/server start
```

Vercel should deploy `site` with:

```text
Root directory: cliks/site
NEXT_PUBLIC_CLIKS_API_URL=https://your-render-service.onrender.com
```

## Install script shape

The public install command will eventually be:

```bash
curl -fsSL https://raw.githubusercontent.com/YashMahawa/Cliks/main/cli/install.sh | bash
```

For now it installs the npm package locally from the repository checkout.
