(function () {
  "use strict";

  // === UI refs ===
  const mainContent = document.getElementById("mainContent");
  const settingsView = document.getElementById("settingsView");
  const statusBar = document.getElementById("statusBar");
  const lastRelayTime = document.getElementById("lastRelayTime");
  const relayNowBtn = document.getElementById("relayNow");
  const toggle = document.getElementById("toggle");
  const settingsBtn = document.getElementById("settingsBtn");
  const backBtn = document.getElementById("backBtn");

  const ablyKeyInput = document.getElementById("ablyKey");
  const ablyChannelInput = document.getElementById("ablyChannel");
  const intervalInput = document.getElementById("interval");
  const saveBtn = document.getElementById("save");
  const saveMsg = document.getElementById("saveMsg");

  const keyStatus = document.getElementById("keyStatus");
  const keyDisplay = document.getElementById("keyDisplay");
  const generateKeyBtn = document.getElementById("generateKey");
  const enterKeyBtn = document.getElementById("enterKey");
  const keyInputRow = document.getElementById("keyInputRow");
  const keyInput = document.getElementById("keyInput");
  const setKeyBtn = document.getElementById("setKey");
  const clearTokensBtn = document.getElementById("clearTokens");
  const clearMsg = document.getElementById("clearMsg");

  // === Status ===

  function renderStatus(status) {
    const t = status.token;
    if (!t) {
      statusBar.innerHTML = '<div class="no-token">Open app.slack.com — token will be captured automatically</div>';
      return;
    }

    const workspaceName = t.teamName || t.teamId || "Unknown workspace";
    const userName = t.userName ? `@${t.userName}` : t.userId || "";
    const badgeClass = t.token ? "ok" : "missing";
    const badgeText = t.token ? "Token valid" : "No token";

    statusBar.innerHTML = `
      <div class="workspace">${escHtml(workspaceName)}</div>
      ${userName ? `<div class="user">${escHtml(userName)}</div>` : ""}
      <span class="token-badge ${badgeClass}">${badgeText}</span>
    `;

    if (status.lastRelay) {
      const ago = Math.round((Date.now() - status.lastRelay) / 1000);
      lastRelayTime.textContent = `Last relayed ${ago < 60 ? ago + "s" : Math.round(ago / 60) + "m"} ago`;
    }

    if (status.enabled) {
      toggle.classList.add("on");
    } else {
      toggle.classList.remove("on");
    }

    relayNowBtn.disabled = !t.token || !status.configured;
  }

  function loadStatus() {
    chrome.runtime.sendMessage({ type: "GET_STATUS" }, (status) => {
      if (!status) return;
      renderStatus(status);

      // Key status
      if (status.hasKey) {
        keyStatus.textContent = "Key active";
        keyStatus.className = "key-status active";
      } else {
        keyStatus.textContent = "No key set — generate or enter one";
        keyStatus.className = "key-status missing";
      }
    });
  }

  // === Config load ===

  function loadConfig() {
    chrome.runtime.sendMessage({ type: "GET_STATUS" }, (status) => {
      if (!status) return;
      if (status.channelName) {
        ablyChannelInput.value = status.channelName;
      }
    });
    chrome.storage.local.get(["configMeta"], (data) => {
      if (data.configMeta?.relayIntervalMs) {
        intervalInput.value = Math.round(data.configMeta.relayIntervalMs / 1000);
      }
    });
  }

  // === Events ===

  settingsBtn.addEventListener("click", () => {
    mainContent.classList.add("hide");
    settingsView.classList.add("show");
    loadConfig();
  });

  backBtn.addEventListener("click", () => {
    mainContent.classList.remove("hide");
    settingsView.classList.remove("show");
    loadStatus();
  });

  relayNowBtn.addEventListener("click", () => {
    relayNowBtn.disabled = true;
    relayNowBtn.textContent = "Relaying...";
    chrome.runtime.sendMessage({ type: "RELAY_NOW" }, () => {
      relayNowBtn.textContent = "Relay Now";
      loadStatus();
    });
  });

  toggle.addEventListener("click", () => {
    const enabled = !toggle.classList.contains("on");
    chrome.runtime.sendMessage({ type: "SET_ENABLED", enabled }, (resp) => {
      if (resp?.enabled) {
        toggle.classList.add("on");
      } else {
        toggle.classList.remove("on");
      }
    });
  });

  saveBtn.addEventListener("click", () => {
    const config = {
      ablyApiKey: ablyKeyInput.value.trim(),
      ablyChannel: ablyChannelInput.value.trim(),
      relayIntervalMs: parseInt(intervalInput.value, 10) * 1000 || 60000,
    };
    if (!config.ablyApiKey || !config.ablyChannel) {
      showMsg(saveMsg, "error", "Service key and channel are required");
      return;
    }
    chrome.runtime.sendMessage({ type: "SAVE_CONFIG", config }, (resp) => {
      if (resp?.status === "saved") {
        showMsg(saveMsg, "success", "Saved");
        ablyKeyInput.value = "";
      } else {
        showMsg(saveMsg, "error", resp?.message || "Save failed");
      }
    });
  });

  generateKeyBtn.addEventListener("click", () => {
    chrome.runtime.sendMessage({ type: "GENERATE_KEY" }, (resp) => {
      if (resp?.key) {
        keyDisplay.textContent = resp.key;
        keyDisplay.classList.add("show");
        keyStatus.textContent = "Key generated — copy it now and enter it in msghub";
        keyStatus.className = "key-status active";
      }
    });
  });

  keyDisplay.addEventListener("click", () => {
    const text = keyDisplay.textContent;
    if (text) {
      navigator.clipboard.writeText(text).catch(() => {});
      const orig = keyDisplay.innerHTML;
      keyDisplay.innerHTML = text + '<div class="hint">Copied!</div>';
      setTimeout(() => { keyDisplay.innerHTML = orig; }, 1500);
    }
  });

  enterKeyBtn.addEventListener("click", () => {
    keyInputRow.classList.toggle("show");
  });

  setKeyBtn.addEventListener("click", () => {
    const key = keyInput.value.trim();
    if (!key) return;
    chrome.runtime.sendMessage({ type: "SET_KEY", key }, (resp) => {
      if (resp?.status === "saved") {
        keyInputRow.classList.remove("show");
        keyInput.value = "";
        keyStatus.textContent = "Key active";
        keyStatus.className = "key-status active";
      }
    });
  });

  clearTokensBtn.addEventListener("click", () => {
    chrome.runtime.sendMessage({ type: "CLEAR_TOKENS" }, () => {
      showMsg(clearMsg, "success", "Cleared");
      loadStatus();
    });
  });

  // === Helpers ===

  function showMsg(el, type, text) {
    el.className = `save-msg ${type}`;
    el.textContent = text;
    setTimeout(() => { el.className = "save-msg"; }, 3000);
  }

  function escHtml(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }

  // Init
  loadStatus();
  setInterval(loadStatus, 5000);
})();
