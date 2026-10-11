import Arborist from '@npmcli/arborist';
import { readFile, realpath } from 'node:fs/promises';

// Arborist resolves graph edges even when npm hoists packages on disk.
export async function analyzeDependencies(target) {
  target = await realpath(target);
  const tree = await new Arborist({ path: target }).loadActual();
  const lock = JSON.parse(await readFile(`${target}/package-lock.json`, 'utf8'));
  const nodes = new Map();
  const edges = new Map();
  const warnings = [];
  const warn = (code, message) => warnings.push({ stage: 'dependencies', code, message });
  // A node can be reached through both production and development paths.
  const seen = new Set();
  const queue = [{ node: tree, dev: false, optional: false, peer: false, path: ['.'] }];
  for (let head = 0; head < queue.length; head++) {
    const state = queue[head];
    const key = `${state.node.location}:${state.dev}:${state.optional}:${state.peer}`;
    if (seen.has(key)) continue;
    seen.add(key);
    if (state.node !== tree) {
      let pkg = nodes.get(state.node.location);
      if (!pkg) {
        const name = state.node.package.name ?? state.node.name ?? '';
        const version = state.node.package.version ?? '';
        const resolved = state.node.resolved ?? lock.packages?.[state.node.location]?.resolved ?? '';
        const queryable = !state.node.isLink &&
          /^https:\/\/registry\.npmjs\.org\//.test(resolved) &&
          /^(?:@[a-z0-9._~-]+\/)?[a-z0-9._~-]+$/i.test(name) &&
          /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(version);
        pkg = {
          id: state.node.location, name, version, location: state.node.location,
          direct: false, queryable, dependency_path: state.path,
          context: { production: false, development: false, optional: false, peer: false, unknown: false },
        };
        nodes.set(pkg.id, pkg);
        if (!queryable) warn('UNSUPPORTED_PACKAGE', `${pkg.id}: package identity/source cannot be queried as a standard npm registry package.`);
        const locked = lock.packages?.[pkg.id];
        if (!locked || locked.version !== version) warn('LOCK_MISMATCH', `${pkg.id}: installed version ${version || '(missing)'} differs from the lockfile inventory.`);
      }
      pkg.context.production ||= !state.dev;
      pkg.context.development ||= state.dev;
      pkg.context.optional ||= state.optional;
      pkg.context.peer ||= state.peer;
    }
    // Linked packages are outside the prototype's supported inventory.
    if (state.node.isLink) continue;
    for (const edge of [...state.node.edgesOut.values()].sort((a, b) => a.name.localeCompare(b.name))) {
      const from = state.node === tree ? '.' : state.node.location;
      const to = edge.to?.location ?? '';
      const edgeKey = `${from}:${edge.name}`;
      if (!edges.has(edgeKey)) {
        edges.set(edgeKey, { from, to, name: edge.name, type: edge.type, valid: edge.valid, error: edge.error ?? (to ? '' : 'MISSING') });
        if (!to || !edge.valid) warn('DEPENDENCY_EDGE', `${from} -> ${edge.name}: ${edge.error ?? 'dependency is not installed'}.`);
      }
      if (!edge.to) continue;
      queue.push({ node: edge.to, dev: state.dev || edge.type === 'dev',
        optional: state.optional || edge.type.includes('optional') || edge.type === 'peerOptional',
        peer: state.peer || edge.type.startsWith('peer'), path: [...state.path, to] });
    }
  }
  for (const edge of tree.edgesOut.values()) {
    if (edge.to && nodes.has(edge.to.location)) nodes.get(edge.to.location).direct = true;
  }
  const extraneous = [...tree.inventory.values()].filter(n => n !== tree && n.extraneous).length;
  if (extraneous) warn('EXTRANEOUS_PACKAGES', `${extraneous} extraneous installed packages are outside the declared dependency graph and were not queried.`);
  const packages = [...nodes.values()].sort((a, b) => a.id.localeCompare(b.id));
  return { tree, packages, edges: [...edges.values()], warnings, extraneous,
    unqueryable: packages.filter(p => !p.queryable).length,
    status: warnings.length ? 'partial' : 'complete' };
}
