import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, readFile, writeFile, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { analyzeDependencies } from '../src/dependencies.mjs';
import { analyzeImports } from '../src/imports.mjs';
import { createProject } from './fixture.mjs';

async function fixture(t, options) {
  const root = await mkdtemp(path.join(tmpdir(), 'patchbot project '));
  t.after(() => rm(root, { recursive: true, force: true }));
  await createProject(root, options);
  return root;
}

test('dependency graph preserves versions, direct edges, dev-only and mixed paths', async t => {
  const root = await fixture(t);
  const result = await analyzeDependencies(root);
  assert.equal(result.status, 'complete');
  const byID = new Map(result.packages.map(p => [p.id, p]));
  assert.equal(byID.get('node_modules/demo-parser').version, '1.0.0');
  assert.equal(byID.get('node_modules/demo-parser').direct, true);
  const nested = byID.get('node_modules/demo-parent/node_modules/demo-parser');
  assert.equal(nested.version, '2.0.0');
  assert.equal(nested.direct, false);
  assert.deepEqual(nested.dependency_path, ['.', 'node_modules/demo-parent', nested.id]);
  assert.equal(byID.get('node_modules/demo-build-tool').context.production, false);
  assert.equal(byID.get('node_modules/demo-build-tool').context.development, true);
  assert.equal(byID.get('node_modules/demo-shared').context.production, true);
  assert.equal(byID.get('node_modules/demo-shared').context.development, true);
  assert.ok(result.packages.every(p => p.queryable));
});

test('source evidence resolves scoped subpaths and only the correct installed version', async t => {
  const root = await fixture(t);
  const deps = await analyzeDependencies(root);
  const result = await analyzeImports(root, deps.tree, deps.packages);
  assert.equal(result.status, 'complete');
  assert.equal(result.scanned, 2);
  assert.ok(result.imports.some(i => i.specifier === '@demo/scoped/tool' && i.node_id === 'node_modules/@demo/scoped'));
  assert.ok(result.imports.some(i => i.kind === 'require' && i.node_id === 'node_modules/demo-build-tool'));
  assert.ok(result.imports.filter(i => i.package_name === 'demo-types').every(i => i.type_only));
  assert.equal(result.imports.find(i => i.package_name === 'demo-parser').line, 1);
  assert.ok(!result.imports.some(i => i.node_id === 'node_modules/demo-parent/node_modules/demo-parser'));
});

test('dynamic loading is explicitly incomplete evidence', async t => {
  const root = await fixture(t, { dynamic: true });
  const deps = await analyzeDependencies(root);
  const result = await analyzeImports(root, deps.tree, deps.packages);
  assert.equal(result.status, 'partial');
  assert.ok(result.warnings.some(w => w.code === 'DYNAMIC_REQUIRE'));
  assert.ok(result.warnings.some(w => w.code === 'DYNAMIC_IMPORT'));
});

test('missing installed dependency and lock mismatch remain visible', async t => {
  const root = await fixture(t);
  await rm(path.join(root, 'node_modules/@demo/scoped'), { recursive: true });
  const lockPath = path.join(root, 'package-lock.json');
  const lock = JSON.parse(await readFile(lockPath, 'utf8'));
  lock.packages['node_modules/demo-parser'].version = '9.0.0';
  await writeFile(lockPath, JSON.stringify(lock));
  const result = await analyzeDependencies(root);
  assert.equal(result.status, 'partial');
  assert.ok(result.warnings.some(w => w.code === 'DEPENDENCY_EDGE'));
  assert.ok(result.warnings.some(w => w.code === 'LOCK_MISMATCH'));
});

test('syntax failures and unresolved packages do not become evidence of non-use', async t => {
  const root = await fixture(t);
  await writeFile(path.join(root, 'src/broken.ts'), 'import { from "missing-package";');
  await writeFile(path.join(root, 'src/unknown.ts'), 'import thing from "not-installed";');
  const deps = await analyzeDependencies(root);
  const result = await analyzeImports(root, deps.tree, deps.packages);
  assert.equal(result.status, 'partial');
  assert.ok(result.warnings.some(w => w.code === 'PARSE_ERROR'));
  assert.ok(result.warnings.some(w => w.code === 'UNRESOLVED_IMPORT'));
});

test('shadowed require and source symlinks are explicit limitations', async t => {
  const root = await fixture(t);
  await writeFile(path.join(root, 'src/shadow.js'), 'function load(require) { require("demo-parent"); }');
  await symlink(path.join(root, 'src/server.ts'), path.join(root, 'src/linked.ts'));
  const deps = await analyzeDependencies(root);
  const result = await analyzeImports(root, deps.tree, deps.packages);
  assert.equal(result.status, 'partial');
  assert.ok(result.warnings.some(w => w.code === 'SHADOWED_REQUIRE'));
  assert.ok(result.warnings.some(w => w.code === 'SOURCE_SYMLINK'));
  assert.ok(!result.imports.some(i => i.file === 'src/shadow.js' || i.file === 'src/linked.ts'));
});
