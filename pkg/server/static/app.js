document.addEventListener("DOMContentLoaded", () => {
  const searchInput = document.getElementById("search-input");
  const searchBtn = document.getElementById("search-btn");
  const resultsContainer = document.getElementById("results-container");
  const modal = document.getElementById("claim-modal");
  const modalClose = document.getElementById("modal-close");
  const modalDomainTitle = document.getElementById("modal-domain-title");
  const recordTypeSelect = document.getElementById("record-type");
  const recordTargetInput = document.getElementById("record-target");
  const recordTargetLabel = document.getElementById("record-target-label");
  const recordTargetHint = document.getElementById("record-target-hint");
  const githubUserInput = document.getElementById("github-username");
  const jsonPreview = document.getElementById("json-preview");
  const submitPrBtn = document.getElementById("submit-pr-btn");
  const expiryText = document.getElementById("expiry-text");

  let currentSubdomain = "";
  let currentZone = "be-a.dev";
  let searchTimeout = null;

  // Search only on Enter or clicking Check
  if (searchInput) {
    searchInput.addEventListener("input", () => {
      if (resultsContainer) resultsContainer.innerHTML = "";
    });

    searchInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        triggerSearch();
      }
    });
  }

  if (searchBtn) {
    searchBtn.addEventListener("click", () => {
      triggerSearch();
    });
  }

  function triggerSearch() {
    if (!searchInput) return;
    const query = sanitizeInput(searchInput.value);
    if (!query) {
      if (resultsContainer) resultsContainer.innerHTML = "";
      return;
    }
    if (query.length < 2) {
      if (resultsContainer) {
        resultsContainer.innerHTML = `<p style="color: var(--text-muted); font-size: 0.84rem; padding: 0.5rem 0;">Subdomain must be at least 2 characters.</p>`;
      }
      return;
    }
    performSearch(query);
  }

  function sanitizeInput(raw) {
    return raw.trim().toLowerCase().replace(/\.?be-a\.dev$/i, "").replace(/[^a-z0-9-]/g, "");
  }

  async function performSearch(query) {
    if (!resultsContainer) return;
    if (searchBtn) {
      searchBtn.disabled = true;
      searchBtn.innerText = "Checking...";
    }
    resultsContainer.innerHTML = `<p style="color: var(--text-muted); font-size: 0.84rem; padding: 0.5rem 0;">Checking ${escapeHtml(query)}.be-a.dev...</p>`;

    try {
      const res = await fetch(`/api/check?name=${encodeURIComponent(query)}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      renderResults(data);
    } catch (err) {
      resultsContainer.innerHTML = `<p style="color: var(--red); font-size: 0.84rem; padding: 0.5rem 0;">Failed to check availability.</p>`;
    } finally {
      if (searchBtn) {
        searchBtn.disabled = false;
        searchBtn.innerText = "Check";
      }
    }
  }

  function renderResults(data) {
    if (!resultsContainer) return;

    if (!data.results || data.results.length === 0) {
      resultsContainer.innerHTML = `<p style="color: var(--text-muted); font-size: 0.84rem; padding: 0.5rem 0;">No results found.</p>`;
      return;
    }

    resultsContainer.innerHTML = data.results.map(r => {
      let dotClass = "dot-avail";
      let stateClass = "state-avail";
      let actionHtml = "";

      if (r.status === "available") {
        dotClass = "dot-avail";
        stateClass = "state-avail";
        actionHtml = `<button class="claim-link-btn" onclick="openClaimModal('${escapeHtml(data.subdomain)}', '${escapeHtml(r.zone)}')">Register &rarr;</button>`;
      } else if (r.status === "taken") {
        dotClass = "dot-taken";
        stateClass = "state-taken";
        actionHtml = `<span style="color: var(--text-muted); font-size: 0.82rem;">Taken</span>`;
      } else {
        dotClass = "dot-reserved";
        stateClass = "state-reserved";
        actionHtml = `<span style="color: var(--text-muted); font-size: 0.82rem;">Reserved</span>`;
      }

      return `
        <div class="result-entry">
          <div>
            <div class="result-domain-line">${escapeHtml(r.fqdn)}</div>
            <div class="result-state ${stateClass}">
              <span class="dot ${dotClass}"></span>
              <span>${escapeHtml(r.message)}</span>
            </div>
          </div>
          <div>${actionHtml}</div>
        </div>
      `;
    }).join("");
  }

  window.openClaimModal = function(subdomain, zone) {
    currentSubdomain = subdomain;
    currentZone = zone || "be-a.dev";
    if (modalDomainTitle) {
      modalDomainTitle.innerText = `${subdomain}.${currentZone}`;
    }
    updateJsonPreview();
    if (modal) {
      modal.classList.add("active");
      if (githubUserInput) githubUserInput.focus();
    }
  };

  function closeModal() {
    if (modal) modal.classList.remove("active");
  }

  if (modalClose) modalClose.addEventListener("click", closeModal);
  if (modal) {
    modal.addEventListener("click", (e) => {
      if (e.target === modal) closeModal();
    });
  }

  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && modal && modal.classList.contains("active")) {
      closeModal();
    }
  });

  if (recordTypeSelect) {
    recordTypeSelect.addEventListener("change", () => {
      const type = recordTypeSelect.value;
      if (type === "CNAME") {
        if (recordTargetLabel) recordTargetLabel.innerText = "Target host";
        if (recordTargetInput) recordTargetInput.placeholder = "username.github.io";
        if (recordTargetHint) recordTargetHint.innerText = "Hostname where traffic should be routed (must end with a dot).";
      } else if (type === "A") {
        if (recordTargetLabel) recordTargetLabel.innerText = "IPv4 address";
        if (recordTargetInput) recordTargetInput.placeholder = "185.12.34.56";
        if (recordTargetHint) recordTargetHint.innerText = "Direct server IPv4 address.";
      } else if (type === "AAAA") {
        if (recordTargetLabel) recordTargetLabel.innerText = "IPv6 address";
        if (recordTargetInput) recordTargetInput.placeholder = "2a01:4f8:...";
        if (recordTargetHint) recordTargetHint.innerText = "Direct server IPv6 address.";
      } else if (type === "TXT") {
        if (recordTargetLabel) recordTargetLabel.innerText = "TXT record";
        if (recordTargetInput) recordTargetInput.placeholder = "verification-string";
        if (recordTargetHint) recordTargetHint.innerText = "Verification token or SPF text.";
      } else if (type === "NS") {
        if (recordTargetLabel) recordTargetLabel.innerText = "Nameservers (comma-separated)";
        if (recordTargetInput) recordTargetInput.placeholder = "ns1.he.net, ns2.he.net";
        if (recordTargetHint) recordTargetHint.innerText = "Delegate DNS management to external nameservers.";
      }
      updateJsonPreview();
    });
  }

  [recordTargetInput, githubUserInput].forEach(el => {
    if (el) el.addEventListener("input", updateJsonPreview);
  });

  function updateJsonPreview() {
    const type = recordTypeSelect ? recordTypeSelect.value : "CNAME";
    const rawTarget = recordTargetInput ? recordTargetInput.value.trim() : "";
    const username = (githubUserInput ? githubUserInput.value.trim() : "") || "username";

    let defaultTarget = "username.github.io";
    if (type === "A") defaultTarget = "185.12.34.56";
    if (type === "AAAA") defaultTarget = "2001:db8::1";
    if (type === "TXT") defaultTarget = "v=spf1 ~all";
    if (type === "NS") defaultTarget = "ns1.he.net, ns2.he.net";

    const target = rawTarget || defaultTarget;
    const recordsObj = {};

    if (type === "CNAME") {
      recordsObj["CNAME"] = target.endsWith(".") ? target : target + ".";
    } else if (type === "A") {
      recordsObj["A"] = [target];
    } else if (type === "AAAA") {
      recordsObj["AAAA"] = [target];
    } else if (type === "TXT") {
      recordsObj["TXT"] = [target];
    } else if (type === "NS") {
      const parts = target.split(",").map(s => s.trim()).filter(Boolean);
      recordsObj["NS"] = parts.length >= 2 ? parts.map(p => p.endsWith(".") ? p : p + ".") : ["ns1.he.net.", "ns2.he.net."];
    }

    const payload = {
      "$schema": "../../../schemas/domain.schema.json",
      "description": `Subdomain for ${username}`,
      "owner": {
        "username": username
      },
      "records": recordsObj,
      "ttl": 300
    };

    const jsonStr = JSON.stringify(payload, null, 2);
    if (jsonPreview) jsonPreview.innerText = jsonStr;

    const shard = currentSubdomain ? currentSubdomain[0] : "a";
    const filePath = `domains/${currentZone}/${shard}/${currentSubdomain}.json`;
    const repoBase = "https://github.com/xdearboy/be-a.dev/new/main";
    const prUrl = `${repoBase}?filename=${encodeURIComponent(filePath)}&value=${encodeURIComponent(jsonStr)}&message=${encodeURIComponent(`Register ${currentSubdomain}.${currentZone}`)}`;
    if (submitPrBtn) submitPrBtn.href = prUrl;
  }

  // Load NS Nodes Status
  async function loadStatus() {
    try {
      const res = await fetch("/api/status");
      if (!res.ok) return;
      const data = await res.json();

      const grid = document.getElementById("status-grid");
      if (data.nameservers && grid) {
        grid.innerHTML = data.nameservers.map(ns => `
          <div class="ns-row">
            <span class="ns-name">${escapeHtml(ns.host)}.${escapeHtml(data.primary_zone || "be-a.dev")}</span>
            <span class="ns-ip">${escapeHtml(ns.ip)}</span>
            <span class="ns-geo">${escapeHtml(ns.region)}</span>
          </div>
        `).join("");
      }

      if (data.expiry && data.expiry.length > 0 && expiryText) {
        const rootExp = data.expiry[0];
        const expDate = rootExp.expires_at ? new Date(rootExp.expires_at).toLocaleDateString("en-US", { month: "short", year: "numeric" }) : "Nov 2027";
        expiryText.innerText = `registered through ${expDate} (${rootExp.days_remaining} days left)`;
      }
    } catch (e) {
      console.warn("Status fetch failed", e);
    }
  }

  function escapeHtml(str) {
    if (!str) return "";
    return str.replace(/[&<>"']/g, m => ({
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#39;"
    })[m]);
  }

  loadStatus();
});
