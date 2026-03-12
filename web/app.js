import { initMap, updateMap, highlightFeature, destroyMap, updateMapTheme } from "./modules/map.js";
import { initScene, updateScene, highlightObject as highlightSceneObject, destroyScene, setWireframe } from "./modules/scene.js";
import { updateSidebar, setSelectionCallback } from "./modules/sidebar.js";

// Hannover city-centre tiles (EPSG:25832 km grid).
// 4 tiles covering the inner city around 550/5800.
const HANNOVER_TILES = [
  { easting: 550, northing: 5800 },
  { easting: 550, northing: 5801 },
  { easting: 551, northing: 5800 },
  { easting: 551, northing: 5801 },
];

let wasmReady = false;
let activeView = "map";
let mergedData = null;

// --- Theme ---

const THEME_KEY = "citygml-viewer-theme";
const THEME_MODES = ["auto", "light", "dark"];
let currentThemeMode = "auto";

function getSystemTheme() {
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyTheme(mode) {
  const theme = mode === "auto" ? getSystemTheme() : mode;
  if (theme === "dark") {
    document.documentElement.setAttribute("data-theme", "dark");
  } else if (mode === "light") {
    document.documentElement.setAttribute("data-theme", "light");
  } else {
    document.documentElement.removeAttribute("data-theme");
  }
  updateMapTheme();
}

function updateThemeToggleButtons() {
  const labels = { auto: "Auto", light: "Light", dark: "Dark" };
  const sunIcon = `
    <circle cx="12" cy="12" r="5"></circle>
    <line x1="12" y1="1" x2="12" y2="3"></line>
    <line x1="12" y1="21" x2="12" y2="23"></line>
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
    <line x1="1" y1="12" x2="3" y2="12"></line>
    <line x1="21" y1="12" x2="23" y2="12"></line>
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
  `;
  const moonIcon = `<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>`;
  const icon = currentThemeMode === "dark" ? moonIcon : sunIcon;

  document.querySelectorAll(".theme-toggle").forEach((btn) => {
    btn.querySelector("svg").innerHTML = icon;
    const labelEl = btn.querySelector(".theme-toggle-label");
    if (labelEl) labelEl.textContent = labels[currentThemeMode];
  });
}

function cycleTheme() {
  const idx = THEME_MODES.indexOf(currentThemeMode);
  currentThemeMode = THEME_MODES[(idx + 1) % THEME_MODES.length];
  applyTheme(currentThemeMode);
  updateThemeToggleButtons();
  localStorage.setItem(THEME_KEY, currentThemeMode);
}

function initTheme() {
  const saved = localStorage.getItem(THEME_KEY);
  if (saved && THEME_MODES.includes(saved)) currentThemeMode = saved;
  applyTheme(currentThemeMode);
  updateThemeToggleButtons();
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (currentThemeMode === "auto") applyTheme("auto");
  });
}

// --- WASM ---

async function initWasm() {
  const go = new Go();
  const result = await WebAssembly.instantiateStreaming(fetch("citygml.wasm"), go.importObject);
  go.run(result.instance);
  wasmReady = true;
}

// --- Tile loading ---

function tileName(easting, northing) {
  return `LoD2_32_${easting}_${northing}_1_ni.gml`;
}

async function loadTile(easting, northing) {
  const name = tileName(easting, northing);
  const resp = await fetch(`tiles/${name}`);
  if (!resp.ok) throw new Error(`Failed to fetch tile ${name}: ${resp.status}`);
  return new Uint8Array(await resp.arrayBuffer());
}

// Merge multiple parsed results into one combined result for display.
function mergeResults(results) {
  // Combine GeoJSON feature collections
  const allFeatures = results.flatMap((r) => (r.geojson?.features ?? []));
  const merged = {
    ...results[0],
    geojson: { type: "FeatureCollection", features: allFeatures },
    meta: {
      ...results[0].meta,
      buildingCount: results.reduce((s, r) => s + (r.meta?.buildingCount ?? 0), 0),
      terrainCount: results.reduce((s, r) => s + (r.meta?.terrainCount ?? 0), 0),
      genericCount: results.reduce((s, r) => s + (r.meta?.genericCount ?? 0), 0),
    },
    objects: results.flatMap((r) => r.objects ?? []),
    findings: results.flatMap((r) => r.findings ?? []),
    scene: {
      objects: results.flatMap((r) => r.scene?.objects ?? []),
    },
  };

  // Merge bounds
  const allBounds = results.map((r) => r.bounds).filter(Boolean);
  if (allBounds.length > 0) {
    const cx = results[0].bounds.centroidX;
    const cy = results[0].bounds.centroidY;
    const cz = results[0].bounds.centroidZ;
    merged.bounds = {
      centroidX: cx,
      centroidY: cy,
      centroidZ: cz,
      minX: Math.min(...allBounds.map((b) => b.minX + b.centroidX)) - cx,
      maxX: Math.max(...allBounds.map((b) => b.maxX + b.centroidX)) - cx,
      minY: Math.min(...allBounds.map((b) => b.minY + b.centroidY)) - cy,
      maxY: Math.max(...allBounds.map((b) => b.maxY + b.centroidY)) - cy,
      minZ: Math.min(...allBounds.map((b) => b.minZ + b.centroidZ)) - cz,
      maxZ: Math.max(...allBounds.map((b) => b.maxZ + b.centroidZ)) - cz,
      has3D: allBounds.some((b) => b.has3D),
    };
  }

  return merged;
}

async function loadHannover() {
  setMessage("Loading WASM…");
  await initWasm();

  const results = [];
  for (let i = 0; i < HANNOVER_TILES.length; i++) {
    const { easting, northing } = HANNOVER_TILES[i];
    setMessage(`Loading tile ${i + 1} / ${HANNOVER_TILES.length} (${easting}/${northing})…`);
    const bytes = await loadTile(easting, northing);

    setMessage(`Parsing tile ${i + 1} / ${HANNOVER_TILES.length}…`);
    // yield to UI
    await new Promise((r) => requestAnimationFrame(r));
    const result = parseCityGML(bytes);
    if (!result.success) throw new Error(result.error || "parse error");
    results.push(result);
  }

  return mergeResults(results);
}

// --- DOM helpers ---

const loadingView = document.getElementById("loading-view");
const vizView = document.getElementById("viz-view");
const splashLoading = document.getElementById("splash-loading");
const splashError = document.getElementById("splash-error");
const splashErrorMsg = document.getElementById("splash-error-message");
const fileMeta = document.getElementById("file-meta");

function setMessage(msg) {
  document.getElementById("splash-message").textContent = msg;
}

function showSplashError(msg) {
  splashLoading.classList.add("hidden");
  splashError.classList.remove("hidden");
  splashErrorMsg.textContent = msg;
}

// --- Boot ---

initTheme();

document.querySelectorAll(".theme-toggle").forEach((btn) => {
  btn.addEventListener("click", cycleTheme);
});

// View toggle
document.querySelectorAll(".viz-toggle-btn").forEach((btn) => {
  btn.addEventListener("click", () => {
    const view = btn.dataset.view;
    if (view === activeView) return;

    document.querySelectorAll(".viz-toggle-btn").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    document.querySelectorAll(".viz-canvas").forEach((c) => c.classList.remove("active"));
    document.getElementById(view === "map" ? "map-container" : "scene-container").classList.add("active");

    activeView = view;
    if (mergedData) {
      if (view === "map") initMap("map", mergedData);
      else initScene("scene-canvas", mergedData);
    }
  });
});

// Sidebar tabs
document.querySelectorAll(".sidebar-tab").forEach((tab) => {
  tab.addEventListener("click", () => {
    document.querySelectorAll(".sidebar-tab").forEach((t) => t.classList.remove("active"));
    tab.classList.add("active");
    document.querySelectorAll(".sidebar-content").forEach((c) => c.classList.remove("active"));
    document.getElementById("tab-" + tab.dataset.tab).classList.add("active");
  });
});

document.getElementById("wireframe-toggle").addEventListener("change", (e) => {
  setWireframe(e.target.checked);
});

setSelectionCallback((objectId) => {
  if (activeView === "map") highlightFeature(objectId);
  else highlightSceneObject(objectId);
});

// Auto-load on page open
loadHannover()
  .then((data) => {
    mergedData = data;

    const parts = [];
    if (data.meta.version) parts.push("v" + data.meta.version);
    if (data.meta.buildingCount) parts.push(data.meta.buildingCount + " buildings");
    fileMeta.textContent = parts.join(" | ");

    loadingView.classList.remove("active");
    vizView.classList.add("active");

    updateSidebar(data);
    initMap("map", data);
  })
  .catch((err) => {
    console.error(err);
    showSplashError("Failed to load Hannover tiles: " + err.message);
  });
