let renderer = null;
let scene = null;
let camera = null;
let controls = null;
let animationId = null;
let meshes = {};
let highlightedMesh = null;
let wireframeMode = false;

const SURFACE_COLORS = {
  RoofSurface: 0xcc3333,
  WallSurface: 0x999999,
  GroundSurface: 0x8b6914,
  Terrain: 0x33aa55,
  Solid: 0x6688cc,
  MultiSurface: 0x6688cc,
};

const HIGHLIGHT_COLOR = 0xf59e0b;

export async function initScene(canvasId, data) {
  if (renderer) {
    updateScene(data);
    return;
  }

  const THREE = await import("three");
  const { OrbitControls } = await import("three/addons/controls/OrbitControls.js");

  const canvas = document.getElementById(canvasId);
  const container = canvas.parentElement;

  renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
  renderer.setPixelRatio(window.devicePixelRatio);
  renderer.setSize(container.clientWidth, container.clientHeight);

  const bgColor = window.matchMedia("(prefers-color-scheme: dark)").matches
    ? 0x0f172a
    : 0xf0f0f0;

  scene = new THREE.Scene();
  scene.background = new THREE.Color(bgColor);

  camera = new THREE.PerspectiveCamera(
    60,
    container.clientWidth / container.clientHeight,
    0.1,
    10000
  );

  controls = new OrbitControls(camera, canvas);
  controls.enableDamping = true;
  controls.dampingFactor = 0.1;

  // Lighting
  scene.add(new THREE.AmbientLight(0xffffff, 0.5));
  const dirLight = new THREE.DirectionalLight(0xffffff, 0.8);
  dirLight.position.set(50, 100, 50);
  scene.add(dirLight);

  buildSceneObjects(THREE, data);
  fitCamera(THREE);

  // Resize observer
  const resizeObserver = new ResizeObserver(() => {
    if (!renderer) return;
    const w = container.clientWidth;
    const h = container.clientHeight;
    renderer.setSize(w, h);
    camera.aspect = w / h;
    camera.updateProjectionMatrix();
  });
  resizeObserver.observe(container);

  // Click handler
  const raycaster = new THREE.Raycaster();
  const mouse = new THREE.Vector2();

  canvas.addEventListener("click", (e) => {
    const rect = canvas.getBoundingClientRect();
    mouse.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    mouse.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;

    raycaster.setFromCamera(mouse, camera);
    const allMeshes = Object.values(meshes).flat();
    const intersects = raycaster.intersectObjects(allMeshes);

    if (intersects.length > 0) {
      const hit = intersects[0].object;
      if (hit.userData.objectId) {
        highlightObject(hit.userData.objectId);
      }
    }
  });

  animate();
}

export function updateScene(data) {
  // Scene is rebuilt on init
}

export function highlightObject(objectId) {
  const THREE = window.__THREE;
  if (!THREE) return;

  // Reset previous highlight
  if (highlightedMesh) {
    for (const m of Object.values(meshes).flat()) {
      if (m.userData.objectId === highlightedMesh) {
        m.material.emissive?.set(0x000000);
      }
    }
  }

  // Apply new highlight
  highlightedMesh = objectId;
  for (const m of Object.values(meshes).flat()) {
    if (m.userData.objectId === objectId && m.material.emissive) {
      m.material.emissive.set(HIGHLIGHT_COLOR);
      m.material.emissiveIntensity = 0.3;
    }
  }
}

export function destroyScene() {
  if (animationId) {
    cancelAnimationFrame(animationId);
    animationId = null;
  }
  if (renderer) {
    renderer.dispose();
    renderer = null;
  }
  scene = null;
  camera = null;
  controls = null;
  meshes = {};
  highlightedMesh = null;
}

export function setWireframe(enabled) {
  wireframeMode = enabled;
  for (const group of Object.values(meshes)) {
    for (const m of group) {
      m.material.wireframe = enabled;
    }
  }
}

async function buildSceneObjects(THREE, data) {
  window.__THREE = THREE;

  if (!data.scene || !data.scene.objects) return;

  for (const obj of data.scene.objects) {
    if (!obj.surfaces) continue;

    const objMeshes = [];

    for (const surf of obj.surfaces) {
      if (!surf.polygons || surf.polygons.length === 0) continue;

      const color = SURFACE_COLORS[surf.type] || 0x888888;

      for (const ring of surf.polygons) {
        if (ring.length < 3) continue;

        const geometry = triangulatePolygon(THREE, ring);
        if (!geometry) continue;

        const material = new THREE.MeshPhongMaterial({
          color: color,
          side: THREE.DoubleSide,
          wireframe: wireframeMode,
          flatShading: true,
        });

        const mesh = new THREE.Mesh(geometry, material);
        mesh.userData.objectId = obj.id;
        mesh.userData.objectType = obj.type;
        scene.add(mesh);
        objMeshes.push(mesh);
      }
    }

    if (objMeshes.length > 0) {
      meshes[obj.id] = objMeshes;
    }
  }
}

function triangulatePolygon(THREE, ring) {
  // Simple ear-clipping fan triangulation for convex-ish polygons
  if (ring.length < 3) return null;

  const vertices = [];
  const indices = [];

  for (const pt of ring) {
    // CityGML UTM coordinates: [easting, northing, height]
    // Three.js is Y-up, so map: X=easting, Y=height, Z=-northing
    vertices.push(pt[0], pt[2], -pt[1]);
  }

  // Fan triangulation from first vertex
  for (let i = 1; i < ring.length - 1; i++) {
    indices.push(0, i, i + 1);
  }

  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute(
    "position",
    new THREE.Float32BufferAttribute(vertices, 3)
  );
  geometry.setIndex(indices);
  geometry.computeVertexNormals();

  return geometry;
}

function fitCamera(THREE) {
  if (!scene) return;

  const box = new THREE.Box3().setFromObject(scene);
  if (box.isEmpty()) return;

  const center = box.getCenter(new THREE.Vector3());
  const size = box.getSize(new THREE.Vector3());
  const maxDim = Math.max(size.x, size.y, size.z);

  camera.position.set(
    center.x + maxDim * 0.8,
    center.y + maxDim * 0.6,
    center.z + maxDim * 0.8
  );
  camera.lookAt(center);
  controls.target.copy(center);
  controls.update();
}

function animate() {
  animationId = requestAnimationFrame(animate);
  if (controls) controls.update();
  if (renderer && scene && camera) {
    renderer.render(scene, camera);
  }
}
