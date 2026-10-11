import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

// Fictional packages: these fixtures never install or execute third-party code.
export async function createProject(target, { dynamic = false } = {}) {
  const manifest = { name: 'patchbot-demo', version: '1.0.0', private: true,
    dependencies: { 'demo-parser': '1.0.0', 'demo-parent': '1.0.0', '@demo/scoped': '1.0.0' },
    devDependencies: { 'demo-build-tool': '1.0.0', 'demo-types': '1.0.0' } };
  const entries = [
    ['node_modules/demo-parser', { name: 'demo-parser', version: '1.0.0', dependencies: { 'demo-shared': '1.0.0' } }],
    ['node_modules/demo-parent', { name: 'demo-parent', version: '1.0.0', dependencies: { 'demo-parser': '2.0.0', 'demo-shared': '1.0.0' } }],
    ['node_modules/demo-parent/node_modules/demo-parser', { name: 'demo-parser', version: '2.0.0' }],
    ['node_modules/demo-shared', { name: 'demo-shared', version: '1.0.0' }],
    ['node_modules/demo-build-tool', { name: 'demo-build-tool', version: '1.0.0', dependencies: { 'demo-shared': '1.0.0' } }],
    ['node_modules/demo-types', { name: 'demo-types', version: '1.0.0' }],
    ['node_modules/@demo/scoped', { name: '@demo/scoped', version: '1.0.0' }],
  ];
  await mkdir(path.join(target, 'src'), { recursive: true });
  await writeFile(path.join(target, 'package.json'), JSON.stringify(manifest));
  const lock = { name: manifest.name, version: manifest.version, lockfileVersion: 3, packages: { '': manifest } };
  for (const [location, pkg] of entries) {
    await mkdir(path.join(target, location), { recursive: true });
    await writeFile(path.join(target, location, 'package.json'), JSON.stringify(pkg));
    lock.packages[location] = { ...pkg, resolved: `https://registry.npmjs.org/${pkg.name}/-/${pkg.name.split('/').pop()}-${pkg.version}.tgz` };
  }
  await writeFile(path.join(target, 'package-lock.json'), JSON.stringify(lock));
  await writeFile(path.join(target, 'src/server.ts'), [
    'import parser from "demo-parser";',
    'import { tool } from "@demo/scoped/tool";',
    'import { type Thing } from "demo-types";',
    'export type { Thing as Other } from "demo-types";',
    ...(dynamic ? ['const packageName = "demo-parser";', 'require(packageName);', 'import("demo-parser");'] : []),
  ].join('\n'));
  await writeFile(path.join(target, 'src/build.js'), 'const build = require("demo-build-tool");\n');
  return target;
}
