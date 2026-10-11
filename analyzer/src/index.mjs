import { realpath, readFile, stat } from 'node:fs/promises';
import { analyzeDependencies } from './dependencies.mjs';
import { analyzeImports } from './imports.mjs';

try {
  if (Number(process.versions.node.split('.')[0]) < 24) throw new Error('PatchBot requires Node.js 24 or newer.');
  if (process.argv.length !== 3) throw new Error('Expected one target project path.');
  const target = await realpath(process.argv[2]);
  const manifest = JSON.parse(await readFile(`${target}/package.json`, 'utf8'));
  const lock = JSON.parse(await readFile(`${target}/package-lock.json`, 'utf8'));
  if (manifest.workspaces) throw new Error('npm workspaces are outside prototype coverage.');
  if (![2, 3].includes(lock.lockfileVersion)) throw new Error('Use an npm package-lock.json with lockfileVersion 2 or 3.');
  if (!(await stat(`${target}/node_modules`)).isDirectory()) throw new Error('Install the target dependencies before scanning.');
  const dependencies = await analyzeDependencies(target);
  let source;
  try { source = await analyzeImports(target, dependencies.tree, dependencies.packages); }
  catch (error) { source = { imports: [], warnings: [{ stage: 'source', code: 'SOURCE_FAILED', message: error.message }],
    scanned: 0, skipped: 0, status: 'failed' }; }
  process.stdout.write(JSON.stringify({ schema_version: 1, inventory_mode: 'installed',
    stages: { dependencies: { status: dependencies.status }, source: { status: source.status } },
    packages: dependencies.packages, edges: dependencies.edges, imports: source.imports,
    coverage: { packages: dependencies.packages.length, extraneous: dependencies.extraneous,
      unqueryable: dependencies.unqueryable, scanned_files: source.scanned, skipped_files: source.skipped,
      queried_versions: 0, failed_queries: 0, withdrawn_advisories: 0 },
    warnings: [...dependencies.warnings, ...source.warnings] }) + '\n');
} catch (error) {
  console.error(`PatchBot analyzer: ${error.message}`);
  process.exitCode = 2;
}
