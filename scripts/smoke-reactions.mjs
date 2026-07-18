import WebSocket from "ws";

const apiUrl = process.env.CLIKS_API_URL?.replace(/\/$/, "");
if (!apiUrl || !/^https?:\/\//.test(apiUrl)) {
  throw new Error("Set CLIKS_API_URL to the relay you intend to test");
}
const wsUrl = apiUrl.replace(/^http/, "ws") + "/ws";
const password = `smoke-${Date.now()}`;
let code = "";

try {
  const created = await fetchJson(`${apiUrl}/api/teams`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ name: "Reaction Smoke", deletePassword: password })
  });
  code = created.team.code;

  const sender = new WebSocket(wsUrl);
  const recipient = new WebSocket(wsUrl);
  await Promise.all([once(sender, "open"), once(recipient, "open")]);

  const received = new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error("recipient did not receive peer_reaction")), 3_000);
    recipient.on("message", (raw) => {
      const message = JSON.parse(raw.toString());
      if (message.type !== "peer_reaction") return;
      clearTimeout(timeout);
      resolve(message);
    });
  });

  sender.send(JSON.stringify({ type: "join", teamCode: code, nickname: "Mira" }));
  recipient.send(JSON.stringify({ type: "join", teamCode: code, nickname: "Noor" }));
  await sleep(250);
  sender.send(JSON.stringify({ type: "reaction", reaction: "break" }));

  const reaction = await received;
  if (reaction.reaction !== "break" || reaction.nickname !== "Mira" || reaction.targetPeerId) {
    throw new Error(`unexpected reaction payload: ${JSON.stringify(reaction)}`);
  }
  sender.close();
  recipient.close();
  console.log(JSON.stringify({ ok: true, relay: apiUrl, reaction: reaction.reaction, nickname: reaction.nickname }));
} finally {
  if (code) {
    await fetch(`${apiUrl}/api/teams/${code}`, {
      method: "DELETE",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ deletePassword: password })
    });
  }
}

async function fetchJson(url, options) {
  const response = await fetch(url, options);
  const body = await response.text();
  if (!response.ok) throw new Error(`${response.status} ${body}`);
  return JSON.parse(body);
}

function once(emitter, event) {
  return new Promise((resolve, reject) => {
    emitter.once(event, resolve);
    emitter.once("error", reject);
  });
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
