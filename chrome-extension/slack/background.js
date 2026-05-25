// Background service worker for the Slack token relay.
// On token capture: reads the xoxs-* session cookie, calls auth.test to get
// workspace/user info, encrypts everything, and publishes to Ably.

importScripts("crypto.js");

let cachedToken = null;        // { token, session, teamId, teamName, userId, userName }
let cachedConfig = null;
let relayEnabled = true;
const DEFAULT_RELAY_INTERVAL_MS = 60 * 1000;

// === Config ===

async function loadConfig() {
  if (cachedConfig) return cachedConfig;

  const [localData, sessionData] = await Promise.all([
    chrome.storage.local.get(["configEncrypted", "configMeta"]),
    chrome.storage.session.get("encryptionKey"),
  ]);

  if (!localData.configEncrypted || !sessionData.encryptionKey) return null;

  try {
    const decrypted = await teamsCLICrypto.decrypt(localData.configEncrypted, sessionData.encryptionKey);
    const sensitive = JSON.parse(decrypted);
    cachedConfig = {
      ...sensitive,
      relayIntervalMs: localData.configMeta?.relayIntervalMs || DEFAULT_RELAY_INTERVAL_MS,
    };
    return cachedConfig;
  } catch (err) {
    console.error("[msghub-slack] config decrypt failed:", err);
    return null;
  }
}

// === Cookie capture ===

async function readSessionCookie() {
  return new Promise((resolve) => {
    chrome.cookies.get({ url: "https://app.slack.com", name: "d" }, (cookie) => {
      resolve(cookie?.value || null);
    });
  });
}

// === Slack auth.test ===

async function authTest(token, session) {
  try {
    const params = new URLSearchParams({ token });
    const headers = { "Content-Type": "application/x-www-form-urlencoded" };
    if (session) headers["Cookie"] = `d=${session}`;

    const resp = await fetch("https://slack.com/api/auth.test", {
      method: "POST",
      headers,
      body: params.toString(),
    });
    if (!resp.ok) return null;
    const data = await resp.json();
    if (!data.ok) return null;
    return {
      teamId: data.team_id || "",
      teamName: data.team || "",
      userId: data.user_id || "",
      userName: data.user || "",
    };
  } catch (e) {
    console.error("[msghub-slack] auth.test failed:", e);
    return null;
  }
}

// === Relay ===

async function relaySlackToken() {
  if (!cachedToken || !relayEnabled) return;

  const config = await loadConfig();
  const sessionData = await chrome.storage.session.get("encryptionKey");
  const key = sessionData.encryptionKey;

  if (!config?.ablyApiKey || !key || !config?.ablyChannel) {
    console.log("[msghub-slack] not configured — skipping relay");
    return;
  }

  try {
    const payload = JSON.stringify(cachedToken);
    const encrypted = await teamsCLICrypto.encrypt(payload, key);

    const resp = await fetch(
      `https://rest.ably.io/channels/${encodeURIComponent(config.ablyChannel)}/messages`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Basic ${btoa(config.ablyApiKey)}`,
        },
        body: JSON.stringify({ name: "slack_token", data: encrypted }),
      }
    );

    if (resp.ok) {
      chrome.storage.local.set({ lastRelayTime: Date.now() });
      console.log("[msghub-slack] token relayed for", cachedToken.teamName || cachedToken.teamId);
    } else {
      const text = await resp.text();
      console.error("[msghub-slack] relay failed:", resp.status, text);
    }
  } catch (err) {
    console.error("[msghub-slack] relay error:", err);
  }
}

// === Message handler ===

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {

  if (message.type === "SLACK_TOKEN_CAPTURED") {
    const { token } = message.payload;

    (async () => {
      const session = await readSessionCookie();
      const info = await authTest(token, session);

      cachedToken = {
        token,
        session: session || "",
        teamId: info?.teamId || "",
        teamName: info?.teamName || "",
        userId: info?.userId || "",
        userName: info?.userName || "",
      };

      chrome.storage.local.set({ latestToken: { ...cachedToken, timestamp: Date.now() } });

      if (relayEnabled) await relaySlackToken();
      sendResponse({ status: "received" });
    })();
    return true;
  }

  if (message.type === "GET_STATUS") {
    (async () => {
      const config = await loadConfig();
      const [localData, sessionData] = await Promise.all([
        chrome.storage.local.get(["latestToken", "lastRelayTime", "relayEnabled"]),
        chrome.storage.session.get("encryptionKey"),
      ]);
      sendResponse({
        configured: !!(config?.ablyApiKey && sessionData.encryptionKey),
        hasToken: !!localData.latestToken,
        lastRelay: localData.lastRelayTime || null,
        channelName: config?.ablyChannel || null,
        enabled: localData.relayEnabled !== false,
        hasKey: !!sessionData.encryptionKey,
        token: localData.latestToken || null,
      });
    })();
    return true;
  }

  if (message.type === "SAVE_CONFIG") {
    chrome.storage.session.get("encryptionKey", async (sessionData) => {
      const encKey = sessionData.encryptionKey;
      if (!encKey) {
        sendResponse({ status: "error", message: "No encryption key — generate or enter one first" });
        return;
      }
      try {
        const sensitive = JSON.stringify({
          ablyApiKey: message.config.ablyApiKey,
          ablyChannel: message.config.ablyChannel,
        });
        const encrypted = await teamsCLICrypto.encrypt(sensitive, encKey);
        chrome.storage.local.set({
          configEncrypted: encrypted,
          configMeta: { relayIntervalMs: message.config.relayIntervalMs },
        }, () => {
          cachedConfig = {
            ablyApiKey: message.config.ablyApiKey,
            ablyChannel: message.config.ablyChannel,
            relayIntervalMs: message.config.relayIntervalMs,
          };
          startRelayLoop();
          sendResponse({ status: "saved" });
        });
      } catch (err) {
        sendResponse({ status: "error", message: err.message });
      }
    });
    return true;
  }

  if (message.type === "GENERATE_KEY") {
    const key = crypto.randomUUID() + "-" + crypto.randomUUID();
    cachedConfig = null;
    chrome.storage.session.set({ encryptionKey: key }, () => {
      sendResponse({ key });
    });
    return true;
  }

  if (message.type === "SET_KEY") {
    cachedConfig = null;
    chrome.storage.session.set({ encryptionKey: message.key }, () => {
      loadConfig().then(() => sendResponse({ status: "saved" }));
    });
    return true;
  }

  if (message.type === "RELAY_NOW") {
    relaySlackToken().then(() => sendResponse({ status: "relayed" }));
    return true;
  }

  if (message.type === "CLEAR_TOKENS") {
    cachedToken = null;
    cachedConfig = null;
    chrome.storage.local.remove(["latestToken", "configEncrypted", "configMeta"]);
    sendResponse({ status: "cleared" });
    return false;
  }

  if (message.type === "SET_ENABLED") {
    relayEnabled = message.enabled;
    chrome.storage.local.set({ relayEnabled });
    if (relayEnabled) startRelayLoop(); else stopRelayLoop();
    sendResponse({ enabled: relayEnabled });
    return false;
  }
});

// === Relay loop ===

function startRelayLoop() {
  loadConfig().then((config) => {
    const intervalMin = (config?.relayIntervalMs || DEFAULT_RELAY_INTERVAL_MS) / 60000;
    chrome.alarms.create("slackRelay", { periodInMinutes: Math.max(intervalMin, 0.5) });
  });
}

function stopRelayLoop() {
  chrome.alarms.clear("slackRelay");
}

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "slackRelay") relaySlackToken();
});

// Resume on service worker startup
chrome.storage.local.get(["latestToken", "relayEnabled"], (data) => {
  if (data.latestToken) {
    const { timestamp, ...tokenData } = data.latestToken;
    cachedToken = tokenData;
  }
  relayEnabled = data.relayEnabled !== false;
  if (relayEnabled) startRelayLoop();
});
