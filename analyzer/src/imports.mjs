import { Project, SyntaxKind, ts } from 'ts-morph';
import { readdir, readFile, realpath } from 'node:fs/promises';
import path from 'node:path';
import { isBuiltin } from 'node:module';

const ignored = new Set(['node_modules', '.git', '.cache', 'dist', 'build', 'coverage']);
const supported = /\.(?:[jt]sx?|[cm]js)$/;

function packageName(specifier) {
  if (specifier.startsWith('.') || specifier.startsWith('/') || specifier.startsWith('#') || isBuiltin(specifier)) return '';
  return specifier.startsWith('@') ? specifier.split('/').slice(0, 2).join('/') : specifier.split('/')[0];
}

export async function analyzeImports(target, tree, packages) {
  target = await realpath(target);
  const files = [];
  const warnings = [];
  let skipped = 0;
  const warn = (code, message) => warnings.push({ stage: 'source', code, message });
  async function walk(dir) {
    for (const entry of (await readdir(dir, { withFileTypes: true })).sort((a, b) => a.name.localeCompare(b.name))) {
      const full = path.join(dir, entry.name);
      const relative = path.relative(target, full).split(path.sep).join('/');
      if (entry.isSymbolicLink()) {
        skipped++;
        warn('SOURCE_SYMLINK', `${relative}: symlink not followed.`);
      } else if (entry.isDirectory()) {
        if (ignored.has(entry.name)) { skipped++; continue; }
        await walk(full);
      } else if (entry.isFile() && supported.test(entry.name) && !/\.d\.ts$/.test(entry.name)) files.push(full);
      else if (entry.isFile()) skipped++;
    }
  }
  await walk(target);
  const project = new Project({ useInMemoryFileSystem: true, skipAddingFilesFromTsConfig: true,
    compilerOptions: { allowJs: true, jsx: ts.JsxEmit.Preserve } });
  const imports = [];
  const known = new Set(packages.map(p => p.id));
  function record(source, node, specifier, kind, typeOnly = false) {
    const file = path.relative(target, source).split(path.sep).join('/');
    const name = packageName(specifier);
    if (!name) {
      if (specifier.startsWith('#')) warn('IMPORT_ALIAS', `${file}: package import aliases are not resolved.`);
      return;
    }
    // Search from the importing file's directory, respecting nested node_modules.
    let current = path.dirname(source);
    let found = null;
    while (current === target || current.startsWith(target + path.sep)) {
      const location = path.relative(target, path.join(current, 'node_modules', name)).split(path.sep).join('/');
      found = tree.inventory.get(location);
      if (found) break;
      if (current === target) break;
      current = path.dirname(current);
    }
    const nodeID = found && known.has(found.location) ? found.location : '';
    if (!nodeID) warn('UNRESOLVED_IMPORT', `${file}: could not associate ${specifier} with a supported dependency node.`);
    imports.push({ specifier, package_name: name, node_id: nodeID, file,
      line: node.getStartLineNumber(), kind, type_only: typeOnly });
  }
  let scanned = 0;
  for (const file of files) {
    const content = await readFile(file, 'utf8');
    // Use the actual extension for JSX/TS parsing; never execute source code.
    const sf = project.createSourceFile(file, content, { overwrite: true });
    if (sf.compilerNode.parseDiagnostics?.length) {
      warn('PARSE_ERROR', `${path.relative(target, file)}: source could not be parsed completely.`);
    }
    scanned++;
    for (const declaration of sf.getImportDeclarations()) {
      const named = declaration.getNamedImports();
      const typeOnly = declaration.isTypeOnly() || (named.length > 0 && named.every(n => n.isTypeOnly()) &&
        !declaration.getDefaultImport() && !declaration.getNamespaceImport());
      record(file, declaration, declaration.getModuleSpecifierValue(), 'import', typeOnly);
    }
    for (const declaration of sf.getExportDeclarations()) {
      const specifier = declaration.getModuleSpecifierValue();
      if (specifier) record(file, declaration, specifier, 'reexport', declaration.isTypeOnly() ||
        (declaration.getNamedExports().length > 0 && declaration.getNamedExports().every(n => n.isTypeOnly())));
    }
    for (const call of sf.getDescendantsOfKind(SyntaxKind.CallExpression)) {
      const expression = call.getExpression();
      if (expression.getKind() === SyntaxKind.ImportKeyword) {
        warn('DYNAMIC_IMPORT', `${path.relative(target, file)}:${call.getStartLineNumber()}: dynamic import is outside static import coverage.`);
      } else if (expression.getText() === 'require') {
        if (expression.getSymbol()?.getDeclarations().some(declaration => declaration.getSourceFile() === sf)) {
          warn('SHADOWED_REQUIRE', `${path.relative(target, file)}:${call.getStartLineNumber()}: locally declared require is not treated as Node.js package loading.`);
          continue;
        }
        const args = call.getArguments();
        if (args.length === 1 && args[0].getKind() === SyntaxKind.StringLiteral) {
          record(file, call, args[0].getLiteralValue(), 'require');
        } else warn('DYNAMIC_REQUIRE', `${path.relative(target, file)}:${call.getStartLineNumber()}: computed require cannot be resolved.`);
      }
    }
    sf.forget();
  }
  return { imports, warnings, scanned, skipped, status: warnings.length ? 'partial' : 'complete' };
}
