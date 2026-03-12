let selectionCallback = null;

export function setSelectionCallback(fn) {
  selectionCallback = fn;
}

export function updateSidebar(data) {
  updateSummary(data);
  updateObjects(data);
  updateValidation(data);
}

function updateSummary(data) {
  const table = document.getElementById("summary-table");
  const meta = data.meta || {};
  const bounds = data.bounds || {};
  const findings = data.findings || [];

  const errors = findings.filter((f) => f.severity === "error").length;
  const warnings = findings.filter((f) => f.severity === "warning").length;

  let rows = [
    ["CityGML Version", meta.version || "Unknown"],
    ["CRS", meta.srsName || "Not declared"],
    ["EPSG Code", meta.epsgCode ? meta.epsgCode : "N/A"],
    ["Buildings", meta.buildingCount || 0],
    ["Terrains", meta.terrainCount || 0],
    ["Generic Objects", meta.genericCount || 0],
  ];

  if (bounds.has3D) {
    rows.push(["3D Extent", "Yes"]);
  }

  if (errors > 0 || warnings > 0) {
    const parts = [];
    if (errors > 0) parts.push(`${errors} error${errors > 1 ? "s" : ""}`);
    if (warnings > 0) parts.push(`${warnings} warning${warnings > 1 ? "s" : ""}`);
    rows.push(["Validation", parts.join(", ")]);
  } else {
    rows.push(["Validation", "No issues"]);
  }

  table.innerHTML = rows
    .map(([label, value]) => `<tr><td>${label}</td><td>${value}</td></tr>`)
    .join("");
}

function updateObjects(data) {
  const list = document.getElementById("objects-list");
  const objects = data.objects || [];

  if (objects.length === 0) {
    list.innerHTML = '<div class="object-item">No objects found</div>';
    return;
  }

  list.innerHTML = objects
    .map((obj) => {
      let detail = "";
      if (obj.height) {
        detail = `${obj.height.toFixed(1)}m (${obj.heightSource || ""})`;
      }
      if (obj.lod) {
        detail += (detail ? " | " : "") + "LoD " + obj.lod;
      }

      return `
        <div class="object-item" data-id="${escapeHtml(obj.id)}" title="${escapeHtml(obj.id)}">
          <div>
            <div class="object-id">${escapeHtml(obj.id || obj.type)}</div>
            ${detail ? `<div class="object-detail">${escapeHtml(detail)}</div>` : ""}
          </div>
          <span class="object-type">${escapeHtml(obj.type)}</span>
        </div>`;
    })
    .join("");

  // Click handlers
  list.querySelectorAll(".object-item").forEach((item) => {
    item.addEventListener("click", () => {
      list.querySelectorAll(".object-item").forEach((i) => i.classList.remove("selected"));
      item.classList.add("selected");

      const id = item.dataset.id;
      if (selectionCallback) selectionCallback(id);
    });
  });
}

function updateValidation(data) {
  const list = document.getElementById("validation-list");
  const findings = data.findings || [];

  const errors = findings.filter((f) => f.severity === "error");
  const warnings = findings.filter((f) => f.severity === "warning");

  let html = "";

  if (findings.length === 0) {
    html += '<div class="validation-summary"><span class="count-ok">No issues found</span></div>';
  } else {
    const parts = [];
    if (errors.length > 0)
      parts.push(`<span class="count-error">${errors.length} error${errors.length > 1 ? "s" : ""}</span>`);
    if (warnings.length > 0)
      parts.push(`<span class="count-warning">${warnings.length} warning${warnings.length > 1 ? "s" : ""}</span>`);
    html += `<div class="validation-summary">${parts.join(", ")}</div>`;
  }

  // Show errors first, then warnings
  const sorted = [...errors, ...warnings];

  html += sorted
    .map(
      (f) => `
      <div class="finding-item ${f.severity}">
        <div class="finding-path">${escapeHtml(f.path)}</div>
        <div class="finding-message">${escapeHtml(f.message)}</div>
      </div>`
    )
    .join("");

  list.innerHTML = html;
}

function escapeHtml(str) {
  if (!str) return "";
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}
