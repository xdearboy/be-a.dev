document.addEventListener("DOMContentLoaded", () => {
  const searchInput = document.getElementById("search-input");
  const searchBtn = document.getElementById("search-btn");
  const resultsContainer = document.getElementById("results-container");
  const modal = document.getElementById("claim-modal");
  const modalClose = document.getElementById("modal-close");
  const modalDomainTitle = document.getElementById("modal-domain-title");
  const recordTypeSelect = document.getElementById("record-type");
  const recordTargetInput = document.getElementById("record-target");
  const githubUserInput = document.getElementById("github-username");
  const jsonPreview = document.getElementById("json-preview");
  const submitPrBtn = document.getElementById("submit-pr-btn");

  let currentSubdomain = "";
  let currentZone = "";
  let searchTimeout = null;

  // Live search debounce
  searchInput.addEventListener("input", () => {
    clearTimeout(searchTimeout);
    const query = searchInput.value.trim().toLowerCase();
    if (!query) {
      resultsContainer.innerHTML = "";
      return;
    }
    searchTimeout = setTimeout(() => performSearch(query), 300);
  });

  searchBtn.addEventListener("click", () => {
    const query = searchInput.value.trim().toLowerCase();
    if (query) performSearch(query);
  });

  async function performSearch(query) {
    try {
      const res = await fetch(`/api/check?name=${encodeURIComponent(query)}`);
      const data = await res.json();
      renderResults(data);
    } catch (err) {
      console.error("Search failed:", err);
    }
  }

  function renderResults(data) {
    if (!data.results || data.results.length === 0) {
      resultsContainer.innerHTML = `<p style="color: var(--text-muted); text-align: center; grid-column: 1/-1;">No results found.</p>`;
      return;
    }

    resultsContainer.innerHTML = data.results.map(r => {
      let badgeClass = "badge-available";
      let btnHtml = "";

      if (r.status === "available") {
        badgeClass = "badge-available";
        btnHtml = `<button class="claim-btn" onclick="openClaimModal('${data.subdomain}', '${r.zone}')">Claim Domain</button>`;
      } else if (r.status === "taken") {
        badgeClass = "badge-taken";
        btnHtml = `<div style="margin-top: 1rem; color: var(--text-muted); font-size: 0.85rem;">Registered by community member</div>`;
      } else {
        badgeClass = "badge-reserved";
        btnHtml = `<div style="margin-top: 1rem; color: var(--text-muted); font-size: 0.85rem;">${r.message || "Reserved name"}</div>`;
      }

      return `
        <div class="result-card">
          <div>
            <div class="result-header">
              <span class="domain-name">${r.fqdn}</span>
              <span class="badge ${badgeClass}">${r.status}</span>
            </div>
            <p style="color: var(--text-secondary); font-size: 0.9rem;">${r.message}</p>
          </div>
          ${btnHtml}
        </div>
      `;
    }).join("");
  }

  window.openClaimModal = function(subdomain, zone) {
    currentSubdomain = subdomain;
    currentZone = zone;
    modalDomainTitle.innerText = `${subdomain}.${zone}`;
    updateJsonPreview();
    modal.classList.add("active");
  };

  modalClose.addEventListener("click", () => modal.classList.remove("active"));
  modal.addEventListener("click", (e) => {
    if (e.target === modal) modal.classList.remove("active");
  });

  [recordTypeSelect, recordTargetInput, githubUserInput].forEach(el => {
    el.addEventListener("input", updateJsonPreview);
  });

  function updateJsonPreview() {
    const type = recordTypeSelect.value;
    const target = recordTargetInput.value.trim() || (type === "CNAME" ? "octocat.github.io" : "185.12.34.56");
    const username = githubUserInput.value.trim() || "your-github-username";

    let recordsObj = {};
    if (type === "CNAME") {
      recordsObj["CNAME"] = target.endsWith(".") ? target : target + ".";
    } else if (type === "A") {
      recordsObj["A"] = [target];
    } else if (type === "NS") {
      const parts = target.split(",").map(s => s.trim()).filter(Boolean);
      recordsObj["NS"] = parts.length >= 2 ? parts : ["ns1.he.net.", "ns2.he.net."];
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
    jsonPreview.innerText = jsonStr;

    // 1-Click GitHub PR link
    const shard = currentSubdomain[0];
    const filePath = `domains/${currentZone}/${shard}/${currentSubdomain}.json`;
    const repoBase = "https://github.com/xdearboy/free-domains/new/main";
    const prUrl = `${repoBase}?filename=${encodeURIComponent(filePath)}&value=${encodeURIComponent(jsonStr)}&message=${encodeURIComponent(`Register ${currentSubdomain}.${currentZone}`)}`;
    submitPrBtn.href = prUrl;
  }

  // Load NS Nodes Status
  async function loadStatus() {
    try {
      const res = await fetch("/api/status");
      const data = await res.json();
      const grid = document.getElementById("status-grid");
      if (data.nameservers && grid) {
        grid.innerHTML = data.nameservers.map(ns => `
          <div class="status-card">
            <div class="ns-header">
              <span>${ns.host}.${data.primary_zone || "kra67.space"}</span>
              <span class="pulse-dot"></span>
            </div>
            <div class="ns-meta">${ns.region} &bull; ${ns.ip}</div>
          </div>
        `).join("");
      }

      const expiryGrid = document.getElementById("expiry-grid");
      if (data.expiry && expiryGrid) {
        expiryGrid.innerHTML = data.expiry.map(e => {
          let badgeClass = "badge-available";
          let badgeText = `${e.days_remaining} days left`;
          if (e.status === "warning") {
            badgeClass = "badge-reserved";
          } else if (e.status === "critical" || e.status === "expired") {
            badgeClass = "badge-taken";
          }
          const expDate = e.expires_at ? new Date(e.expires_at).toLocaleDateString() : "N/A";
          return `
            <div class="status-card">
              <div class="ns-header">
                <span>${e.domain}</span>
                <span class="badge ${badgeClass}">${badgeText}</span>
              </div>
              <div class="ns-meta">Expires on ${expDate} &bull; via ${e.source ? e.source.toUpperCase() : "RDAP"}</div>
            </div>
          `;
        }).join("");
      }
    } catch (e) {
      console.warn("Status fetch failed", e);
    }
  }

  loadStatus();
});
