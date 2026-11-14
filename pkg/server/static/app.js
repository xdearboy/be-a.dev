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
  const digCommand = document.getElementById("dig-command");

  let currentSubdomain = "";
  let currentZone = "be-a.dev";
  let searchTimeout = null;

  // Copy dig command
  if (copyDigBtn && digCommand) {
    copyDigBtn.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(digCommand.innerText.trim());
        copyDigBtn.innerText = "Copied!";
        setTimeout(() => {
          copyDigBtn.innerText = "Copy";
        }, 2000);
      } catch (err) {
        console.warn("clipboard copy failed", err);
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
      searchTimeout = setTimeout(() => performSearch(query), 200);
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
    resultsContainer.innerHTML = `<p style="color: var(--text-muted); font-size: 0.9rem;">Checking availability...</p>`;

    try {
      const res = await fetch(`/api/check?name=${encodeURIComponent(query)}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      renderResults(data);
    } catch (err) {
      resultsContainer.innerHTML = `<p style="color: var(--red); font-size: 0.9rem;">Failed to check availability. Please try again.</p>`;
    }
  }

  function renderResults(data) {
    if (!resultsContainer) return;

    if (!data.results || data.results.length === 0) {
      resultsContainer.innerHTML = `<p style="color: var(--text-muted); font-size: 0.9rem;">No results found for "${escapeHtml(data.subdomain || "")}"</p>`;
      return;
    }

    resultsContainer.innerHTML = data.results.map(r => {
      let pillHtml = "";
      let actionHtml = "";

      if (r.status === "available") {
        pillHtml = `<span class="status-pill status-available">Available</span>`;
        actionHtml = `<button class="btn btn-primary" onclick="openClaimModal('${escapeHtml(data.subdomain)}', '${escapeHtml(r.zone)}')">Claim</button>`;
      } else if (r.status === "taken") {
        pillHtml = `<span class="status-pill status-taken">Registered</span>`;
        actionHtml = `<span style="color: var(--text-muted); font-size: 0.85rem;">Unavailable</span>`;
      } else {
        pillHtml = `<span class="status-pill status-reserved">Reserved</span>`;
        actionHtml = `<span style="color: var(--text-muted); font-size: 0.85rem;">Protected</span>`;
      }

      return `
        <div class="result-card">
          <div>
            <div style="display: flex; align-items: center; gap: 0.6rem;">
              <span class="result-name">${escapeHtml(r.fqdn)}</span>
              ${pillHtml}
            </div>
            <div class="result-meta">${escapeHtml(r.message)}</div>
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
        if (recordTargetLabel) recordTargetLabel.innerText = "Target Host";
        if (recordTargetInput) recordTargetInput.placeholder = "octocat.github.io";
        if (recordTargetHint) recordTargetHint.innerText = "Target hostname for GitHub Pages, Vercel, Fly.io, etc.";
      } else if (type === "A") {
        if (recordTargetLabel) recordTargetLabel.innerText = "IPv4 Address";
        if (recordTargetInput) recordTargetInput.placeholder = "185.12.34.56";
        if (recordTargetHint) recordTargetHint.innerText = "Dedicated VPS or server IPv4 address.";
      } else if (type === "AAAA") {
        if (recordTargetLabel) recordTargetLabel.innerText = "IPv6 Address";
        if (recordTargetInput) recordTargetInput.placeholder = "2a01:4f8:...";
        if (recordTargetHint) recordTargetHint.innerText = "Direct server IPv6 address.";
      } else if (type === "TXT") {
        if (recordTargetLabel) recordTargetLabel.innerText = "TXT Value";
        if (recordTargetInput) recordTargetInput.placeholder = "verification-string";
        if (recordTargetHint) recordTargetHint.innerText = "Verification or configuration text.";
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

    let defaultTarget = "octocat.github.io";
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
          <tr>
            <td><code>${ns.host}.${data.primary_zone || "be-a.dev"}</code></td>
            <td><code>${ns.ip}</code></td>
            <td>${ns.region}</td>
            <td><span class="status-pill status-active">Active</span></td>
          </tr>
        `).join("");
      }

      const expiryGrid = document.getElementById("expiry-grid");
      if (data.expiry && expiryGrid) {
        expiryGrid.innerHTML = data.expiry.map(e => {
          const expDate = e.expires_at ? new Date(e.expires_at).toLocaleDateString("en-US", { year: "numeric", month: "long", day: "numeric" }) : "November 13, 2027";
          return `
            <div class="health-row">
              <div>
                <strong>${e.domain}</strong>
                <div class="health-detail">Registered until ${expDate} via Google Registry</div>
              </div>
              <span class="status-pill status-active">${e.days_remaining} days remaining</span>
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
