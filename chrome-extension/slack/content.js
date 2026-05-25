// Content script for app.slack.com.
// Injects the MAIN world interceptor to capture the xoxc-* token,
// then forwards it to the background worker along with the xoxs-* session cookie.

(function () {
  "use strict";

  console.log("[msghub-slack] content script loaded");

  let lastToken = null;

  // Inject MAIN world script to intercept fetch/XHR
  const script = document.createElement("script");
  script.src = chrome.runtime.getURL("injector.js");
  script.onload = () => script.remove();
  (document.head || document.documentElement).appendChild(script);

  // Receive token from injected script
  window.addEventListener("message", (event) => {
    if (event.source !== window) return;
    if (event.data?.type !== "__MSGHUB_SLACK_TOKEN__") return;

    const token = event.data.token;
    if (!token || token === lastToken) return;

    lastToken = token;
    console.log("[msghub-slack] token captured:", token.slice(0, 12) + "...");

    chrome.runtime.sendMessage({
      type: "SLACK_TOKEN_CAPTURED",
      payload: { token, url: location.href },
    });
  });
})();
