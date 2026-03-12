let map = null;
let popup = null;

export function initMap(containerId, data) {
  if (map) {
    updateMap(data);
    map.resize();
    return;
  }

  const geojson = reprojectGeoJSON(data.geojson, data.meta?.epsgCode);
  if (!geojson || !geojson.features || geojson.features.length === 0) {
    return;
  }

  map = new maplibregl.Map({
    container: containerId,
    style: {
      version: 8,
      sources: {},
      layers: [
        {
          id: "background",
          type: "background",
          paint: { "background-color": getBackgroundColor() },
        },
      ],
    },
    center: [0, 0],
    zoom: 1,
  });

  popup = new maplibregl.Popup({ closeButton: true, closeOnClick: false });

  map.on("load", () => {
    addDataToMap(geojson, data);
    fitBounds(geojson);
  });
}

export function updateMap(data) {
  if (!map) return;
  const source = map.getSource("citygml");
  if (source) {
    source.setData(reprojectGeoJSON(data.geojson, data.meta?.epsgCode));
  }
}

export function highlightFeature(objectId) {
  if (!map) return;
  map.setFilter("buildings-highlight", ["==", ["id"], objectId]);
}

export function destroyMap() {
  if (map) {
    map.remove();
    map = null;
    popup = null;
  }
}

function getBackgroundColor() {
  return document.documentElement.getAttribute("data-theme") === "dark"
    ? "#1e293b"
    : "#f0f0f0";
}

export function updateMapTheme() {
  if (!map) return;
  map.setPaintProperty("background", "background-color", getBackgroundColor());
}

function addDataToMap(geojson, data) {
  map.addSource("citygml", {
    type: "geojson",
    data: geojson,
    promoteId: "id",
  });

  // Building fill
  map.addLayer({
    id: "buildings-fill",
    type: "fill",
    source: "citygml",
    filter: ["==", ["get", "type"], "Building"],
    paint: {
      "fill-color": buildingColorExpression(data),
      "fill-opacity": 0.6,
    },
  });

  // Building outline
  map.addLayer({
    id: "buildings-outline",
    type: "line",
    source: "citygml",
    filter: ["==", ["get", "type"], "Building"],
    paint: {
      "line-color": "#1e40af",
      "line-width": 1,
    },
  });

  // Building highlight
  map.addLayer({
    id: "buildings-highlight",
    type: "line",
    source: "citygml",
    filter: ["==", ["id"], ""],
    paint: {
      "line-color": "#f59e0b",
      "line-width": 3,
    },
  });

  // Terrain fill
  map.addLayer({
    id: "terrain-fill",
    type: "fill",
    source: "citygml",
    filter: ["==", ["get", "type"], "Terrain"],
    paint: {
      "fill-color": "#22c55e",
      "fill-opacity": 0.3,
    },
  });

  // Terrain outline
  map.addLayer({
    id: "terrain-outline",
    type: "line",
    source: "citygml",
    filter: ["==", ["get", "type"], "Terrain"],
    paint: {
      "line-color": "#16a34a",
      "line-width": 1,
    },
  });

  // Click handler
  map.on("click", "buildings-fill", (e) => {
    if (e.features.length === 0) return;
    const f = e.features[0];
    const props = f.properties;

    let html = `<strong>${f.id || "Building"}</strong><br>`;
    if (props.class) html += `Class: ${props.class}<br>`;
    if (props.function) html += `Function: ${props.function}<br>`;
    if (props.measuredHeight) html += `Height: ${props.measuredHeight}m (measured)<br>`;
    else if (props.derivedHeight) html += `Height: ${props.derivedHeight}m (derived)<br>`;
    if (props.lod) html += `LoD: ${props.lod}<br>`;

    popup.setLngLat(e.lngLat).setHTML(html).addTo(map);

    map.setFilter("buildings-highlight", ["==", ["id"], f.id || ""]);
  });

  map.on("click", "terrain-fill", (e) => {
    if (e.features.length === 0) return;
    const f = e.features[0];
    popup
      .setLngLat(e.lngLat)
      .setHTML(`<strong>${f.id || "Terrain"}</strong>`)
      .addTo(map);
  });

  // Cursor
  map.on("mouseenter", "buildings-fill", () => {
    map.getCanvas().style.cursor = "pointer";
  });
  map.on("mouseleave", "buildings-fill", () => {
    map.getCanvas().style.cursor = "";
  });
}

function buildingColorExpression(data) {
  // Color by height if available
  const heights = (data.objects || [])
    .filter((o) => o.type === "Building" && o.height > 0)
    .map((o) => o.height);

  if (heights.length === 0) {
    return "#3b82f6";
  }

  const minH = Math.min(...heights);
  const maxH = Math.max(...heights);

  if (minH === maxH) {
    return "#3b82f6";
  }

  return [
    "interpolate",
    ["linear"],
    ["coalesce", ["get", "measuredHeight"], ["get", "derivedHeight"], 0],
    minH,
    "#3b82f6",
    (minH + maxH) / 2,
    "#f59e0b",
    maxH,
    "#ef4444",
  ];
}

function fitBounds(geojson) {
  if (!map || !geojson.features.length) return;

  const bounds = new maplibregl.LngLatBounds();
  let hasCoords = false;

  for (const feature of geojson.features) {
    if (!feature.geometry) continue;
    visitCoords(feature.geometry.coordinates, (lng, lat) => {
      // Only add reasonable coordinates (WGS84 range)
      if (Math.abs(lng) <= 180 && Math.abs(lat) <= 90) {
        bounds.extend([lng, lat]);
        hasCoords = true;
      }
    });
  }

  if (hasCoords) {
    map.fitBounds(bounds, { padding: 40, maxZoom: 18 });
  }
}

function visitCoords(coords, fn) {
  if (!Array.isArray(coords)) return;
  if (typeof coords[0] === "number") {
    fn(coords[0], coords[1]);
    return;
  }
  for (const c of coords) {
    visitCoords(c, fn);
  }
}

// UTM EPSG codes: 25831-25838 (ETRS89) and 32601-32660 (WGS84 northern)
function utmZoneFromEPSG(epsg) {
  if (epsg >= 25831 && epsg <= 25838) return { zone: epsg - 25800, north: true };
  if (epsg >= 32601 && epsg <= 32660) return { zone: epsg - 32600, north: true };
  if (epsg >= 32701 && epsg <= 32760) return { zone: epsg - 32700, north: false };
  return null;
}

// Convert UTM easting/northing to WGS84 [longitude, latitude].
// Based on standard Transverse Mercator series expansion (Bowring/USGS).
function utmToLonLat(easting, northing, zone, isNorth) {
  const a = 6378137.0;
  const e2 = 0.00669437999014; // WGS84 eccentricity squared
  const k0 = 0.9996;

  const x = easting - 500000;
  const y = isNorth ? northing : northing - 10000000;

  const lon0 = ((zone - 1) * 6 - 180 + 3) * (Math.PI / 180);

  const e4 = e2 * e2;
  const e6 = e4 * e2;

  const M = y / k0;
  const mu =
    M /
    (a *
      (1 - e2 / 4 - (3 * e4) / 64 - (5 * e6) / 256));

  const e1 = (1 - Math.sqrt(1 - e2)) / (1 + Math.sqrt(1 - e2));
  const phi1 =
    mu +
    ((3 * e1) / 2 - (27 * e1 * e1 * e1) / 32) * Math.sin(2 * mu) +
    ((21 * e1 * e1) / 16 - (55 * e1 * e1 * e1 * e1) / 32) * Math.sin(4 * mu) +
    ((151 * e1 * e1 * e1) / 96) * Math.sin(6 * mu) +
    ((1097 * e1 * e1 * e1 * e1) / 512) * Math.sin(8 * mu);

  const sinPhi1 = Math.sin(phi1);
  const cosPhi1 = Math.cos(phi1);
  const tanPhi1 = Math.tan(phi1);

  const N1 = a / Math.sqrt(1 - e2 * sinPhi1 * sinPhi1);
  const T1 = tanPhi1 * tanPhi1;
  const C1 = (e2 / (1 - e2)) * cosPhi1 * cosPhi1;
  const R1 = (a * (1 - e2)) / Math.pow(1 - e2 * sinPhi1 * sinPhi1, 1.5);
  const D = x / (N1 * k0);
  const D2 = D * D;
  const D4 = D2 * D2;
  const D6 = D4 * D2;

  const lat =
    phi1 -
    ((N1 * tanPhi1) / R1) *
      (D2 / 2 -
        ((5 + 3 * T1 + 10 * C1 - 4 * C1 * C1 - 9 * (e2 / (1 - e2))) * D4) / 24 +
        ((61 + 90 * T1 + 298 * C1 + 45 * T1 * T1 - 252 * (e2 / (1 - e2)) - 3 * C1 * C1) * D6) / 720);

  const lon =
    lon0 +
    (D -
      ((1 + 2 * T1 + C1) * D2 * D) / 6 +
      ((5 - 2 * C1 + 28 * T1 - 3 * C1 * C1 + 8 * (e2 / (1 - e2)) + 24 * T1 * T1) * D4 * D) / 120) /
      cosPhi1;

  return [lon * (180 / Math.PI), lat * (180 / Math.PI)];
}

function transformCoord(coord, utmInfo) {
  if (!utmInfo) return coord;
  return utmToLonLat(coord[0], coord[1], utmInfo.zone, utmInfo.north);
}

function transformCoords(coords, utmInfo) {
  if (!Array.isArray(coords)) return coords;
  if (typeof coords[0] === "number") return transformCoord(coords, utmInfo);
  return coords.map((c) => transformCoords(c, utmInfo));
}

function reprojectGeoJSON(geojson, epsgCode) {
  if (!geojson) return geojson;
  const utmInfo = epsgCode ? utmZoneFromEPSG(epsgCode) : null;
  if (!utmInfo) return geojson;

  return {
    ...geojson,
    features: geojson.features.map((f) => ({
      ...f,
      geometry: f.geometry
        ? { ...f.geometry, coordinates: transformCoords(f.geometry.coordinates, utmInfo) }
        : null,
    })),
  };
}
