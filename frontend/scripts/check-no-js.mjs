// Frontend.md §1 principle 4: TypeScript only. Fails if src/ holds .js / .jsx.
import { readdirSync } from 'node:fs';
import { join } from 'node:path';

const bad = [];
const walk = (dir) => {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) walk(p);
    else if (/\.(js|jsx)$/.test(e.name)) bad.push(p);
  }
};
walk('src');
if (bad.length) {
  console.error(`JavaScript files are not allowed in src/:\n${bad.join('\n')}`);
  process.exit(1);
}
