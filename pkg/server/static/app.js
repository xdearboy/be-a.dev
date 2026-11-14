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
  const copyDigBtn = document.getElementById("copy-dig-btn");
  const copyDigText = document.getElementById("copy-dig-text");
  const digCommand = document.getElementById("dig-command");

  let currentSubdomain = "";
  let currentZone = "be-a.dev";
  let searchTimeout = null;

  // Copy dig command
  if (copyDigBtn && digCommand) {
    copyDigBtn.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(digCommand.innerText.trim());
        if (copyDigText) copyDigText.innerText = "Copied!";
        copyDigBtn.classList.add("copied");
        setTimeout(() => {
          if (copyDigText) copyDigText.innerText = "Copy";
          copyDigBtn.classList.remove("copied");
        }, 2000);
      } catch (err) {
        console.warn("Clipboard write failed", err);
      }
    });
  }

  // Live search debounce
  if (searchInput) {
    searchInput.addEventListener("input", () => {
      clearTimeout(searchTimeout);
      const query = sanitizeInput(searchInput.value);
      if (!query) {
        resultsContainer.innerHTML = "";
        return;
      }
      searchTimeout = setTimeout(() => performSearch(query), 250);
    });

    searchInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        clearTimeout(searchTimeout);
        const query = sanitizeInput(searchInput.value);
        if (query) performSearch(query);
      }
    });
  }

  if (searchBtn) {
    searchBtn.addEventListener("click", () => {
      const query = sanitizeInput(searchInput.value);
      if (query) performSearch(query);
    });
  }

  function sanitizeInput(raw) {
    return raw.trim().toLowerCase().replace(/[^a-z0-9-]/g, "");
  }

  async function performSearch(query) {
    if (!resultsContainer) return;
    resultsContainer.innerHTML = `
      <div class="search-loading">
        <span class="spinner"></span>
        <span>Checking availability across Geo-DNS registry...</span>
      </div>
    `;

    try {
      const res = await fetch(`/api/check?name=${encodeURIComponent(query)}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      renderResults(data);
    } catch (err) {
      console.error("Search failed:", err);
      resultsContainer.innerHTML = `
        <div class="search-error">
          <span>Failed to query registry API. Please retry.</span>
        </div>
      `;
    }
  }

  function renderResults(data) {
    if (!resultsContainer) return;

    if (!data.results || data.results.length === 0) {
      resultsContainer.innerHTML = `
        <div class="search-empty">
          <span>No results for "<strong>${escapeHtml(data.subdomain || "")}</strong>"</span>
        </div>
      `;
      return;
    }

    resultsContainer.innerHTML = data.results.map(r => {
      let badgeClass = "badge-available";
      let actionHtml = "";
      const isAvailable = r.status === "available";

      if (isAvailable) {
        badgeClass = "badge-available";
        actionHtml = `
          <button class="primary-btn claim-action-btn" onclick="openClaimModal('${escapeHtml(data.subdomain)}', '${escapeHtml(r.zone)}')">
            <span>Claim Domain</span>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M5 12h14M12 5l7 7-7 7"/></svg>
          </button>
        `;
      } else if (r.status === "taken") {
        badgeClass = "badge-taken";
        actionHtml = `
          <span class="status-note taken">Active community domain</span>
        `;
      } else {
        badgeClass = "badge-reserved";
        actionHtml = `
          <span class="status-note reserved">${escapeHtml(r.message || "Reserved")}</span>
        `;
      }

      return `
        <div class="result-card ${r.status}">
          <div class="result-card-main">
            <div class="result-header">
              <span class="domain-name">${escapeHtml(r.fqdn)}</span>
              <span class="badge ${badgeClass}">${escapeHtml(r.status)}</span>
            </div>
            <p class="result-desc">${escapeHtml(r.message)}</p>
          </div>
          <div class="result-card-actions">
            ${actionHtml}
          </div>
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
      document.body.style.overflow = "hidden";
      if (githubUserInput) githubUserInput.focus();
    }
  };

  function closeModal() {
    if (modal) {
      modal.classList.remove("active");
      document.body.style.overflow = "";
    }
  }

  if (modalClose) {
    modalClose.addEventListener("click", closeModal);
  }

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

  // Dynamic hints depending on record type
  if (recordTypeSelect) {
    recordTypeSelect.addEventListener("change", () => {
      const type = recordTypeSelect.value;
      if (type === "CNAME") {
        if (recordTargetLabel) recordTargetLabel.innerText = "Target Host (CNAME)";
        if (recordTargetInput) recordTargetInput.placeholder = "username.github.io";
        if (recordTargetHint) recordTargetHint.innerText = "Target FQDN for GitHub Pages, Vercel, Fly.io, Cloudflare, etc.";
      } else if (type === "A") {
        if (recordTargetLabel) recordTargetLabel.innerText = "IPv4 Address (A)";
        if (recordTargetInput) recordTargetInput.placeholder = "185.12.34.56";
        if (recordTargetHint) recordTargetHint.innerText = "Direct IPv4 address of your dedicated server or VPS.";
      } else if (type === "AAAA") {
        if (recordTargetLabel) recordTargetLabel.innerText = "IPv6 Address (AAAA)";
        if (recordTargetInput) recordTargetInput.placeholder = "2a01:4f8:...";
        if (recordTargetHint) recordTargetHint.innerText = "Direct IPv6 address of your server.";
      } else if (type === "TXT") {
        if (recordTargetLabel) recordTargetLabel.innerText = "Text Record (TXT)";
        if (recordTargetInput) recordTargetInput.placeholder = "google-site-verification=...";
        if (recordTargetHint) recordTargetHint.innerText = "Arbitrary verification or configuration string.";
      } else if (type === "NS") {
        if (recordTargetLabel) recordTargetLabel.innerText = "Nameservers (comma-separated)";
        if (recordTargetInput) recordTargetInput.placeholder = "ns1.he.net, ns2.he.net";
        if (recordTargetHint) recordTargetHint.innerText = "Delegate subdomain DNS management to external nameservers.";
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
    const username = (githubUserInput ? githubUserInput.value.trim() : "") || "your-github-username";

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
      "description": `Free subdomain for ${username}`,
      "owner": {
        "username": username
      },
      "records": recordsObj,
      "ttl": 300
    };

    const jsonStr = JSON.stringify(payload, null, 2);
    if (jsonPreview) jsonPreview.innerText = jsonStr;

    // GitHub 1-Click PR URL
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
        const flagMap = { "ns1": "🇳🇱", "ns2": "🇩🇪", "ns3": "🇵🇱" };
        grid.innerHTML = data.nameservers.map(ns => {
          const flag = flagMap[ns.host] || "🌐";
          return `
            <div class="node-card">
              <div class="node-top">
                <div class="node-host"><span class="flag">${flag}</span> ${ns.host}.${data.primary_zone || "be-a.dev"}</div>
                <span class="live-indicator"><span class="live-dot"></span> Active</span>
              </div>
              <div class="node-ip">${ns.ip}</div>
              <div class="node-meta">${ns.region} &bull; Authoritative CoreDNS</div>
            </div>
          `;
        }).join("");
      }

      const expiryGrid = document.getElementById("expiry-grid");
      if (data.expiry && expiryGrid) {
        expiryGrid.innerHTML = data.expiry.map(e => {
          let badgeClass = "healthy";
          let badgeText = `${e.days_remaining} Days Remaining`;
          let fillWidth = "100%";

          if (e.status === "warning") {
            badgeClass = "warning";
            badgeText = `${e.days_remaining} Days Remaining (Renewal Warning)`;
            fillWidth = "25%";
          } else if (e.status === "critical" || e.status === "expired") {
            badgeClass = "critical";
            badgeText = `${e.days_remaining} Days (Action Required)`;
            fillWidth = "5%";
          }

          const expDate = e.expires_at ? new Date(e.expires_at).toLocaleDateString("en-US", { year: "numeric", month: "long", day: "numeric" }) : "November 13, 2027";
          return `
            <div class="expiry-card">
              <div class="expiry-main">
                <div>
                  <div class="expiry-domain">${e.domain}</div>
                  <div class="expiry-info">Registered until ${expDate} &bull; Google Registry</div>
                </div>
                <div class="expiry-badge ${badgeClass}">${badgeText}</div>
              </div>
              <div class="progress-bar-bg">
                <div class="progress-bar-fill ${badgeClass}" style="width: ${fillWidth};"></div>
              </div>
            </div>
          `;
        }).join("");
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
