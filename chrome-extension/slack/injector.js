// Injected into the MAIN world to passively observe Slack API fetch/XHR calls.
// Extracts xoxc-*/xoxd-* tokens and posts them to the content script.
// Never modifies request arguments — read-only observation only.

(function () {
  "use strict";

  const TOKEN_RE = /xox[cd]-[A-Za-z0-9\-]+/;
  const FORM_TOKEN_RE = /(?:^|&)token=(xox[cd]-[^&]+)/;

  const originalFetch = window.fetch;

  window.fetch = function (...args) {
    try {
      const url = typeof args[0] === "string" ? args[0] : args[0]?.url || "";
      if (isSlackApi(url)) {
        observeInit(args[1] || {});
      }
    } catch (_) {}
    return originalFetch.apply(this, args);
  };

  const originalOpen = XMLHttpRequest.prototype.open;
  const originalSend = XMLHttpRequest.prototype.send;

  XMLHttpRequest.prototype.open = function (method, url, ...rest) {
    this._msghubUrl = url;
    return originalOpen.call(this, method, url, ...rest);
  };

  XMLHttpRequest.prototype.send = function (body) {
    try {
      if (this._msghubUrl && isSlackApi(this._msghubUrl) && typeof body === "string") {
        extractFromString(body);
      }
    } catch (_) {}
    return originalSend.call(this, body);
  };

  function isSlackApi(url) {
    return url.includes("slack.com/api/") || url.includes("edgeapi.slack.com/");
  }

  function observeInit(init) {
    // Authorization header
    try {
      const h = init.headers;
      let auth = null;
      if (h instanceof Headers) {
        auth = h.get("Authorization") || h.get("authorization");
      } else if (h && typeof h === "object") {
        auth = h["Authorization"] || h["authorization"] || null;
      }
      if (auth) {
        const m = auth.match(TOKEN_RE);
        if (m) { postToken(m[0]); return; }
      }
    } catch (_) {}

    // Body: string (form-encoded or JSON), URLSearchParams
    try {
      const body = init.body;
      if (typeof body === "string") {
        extractFromString(body);
      } else if (body instanceof URLSearchParams) {
        extractFromString(body.toString());
      }
    } catch (_) {}
  }

  function extractFromString(str) {
    // Form-encoded: token=xoxd-...
    const fm = str.match(FORM_TOKEN_RE);
    if (fm) { postToken(decodeURIComponent(fm[1])); return; }

    // JSON: "token":"xoxd-..." or {"token":"xoxd-..."}
    const jm = str.match(/"token"\s*:\s*"(xox[cd]-[^"]+)"/);
    if (jm) { postToken(jm[1]); return; }

    // Any xoxc/xoxd token anywhere in the string (last resort)
    const am = str.match(TOKEN_RE);
    if (am) postToken(am[0]);
  }

  function postToken(token) {
    console.debug("[msghub] token observed:", token.slice(0, 14) + "...");
    window.postMessage({ type: "__MSGHUB_SLACK_TOKEN__", token }, "*");
  }
})();
