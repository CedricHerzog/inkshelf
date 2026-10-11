// Compiles MDX pages without building the site, to catch syntax errors fast (several people,
// or agents, can run it at once; npm run build cannot share its .docusaurus folder).
// Usage: node scripts/check-mdx.mjs docs/screen/eink.mdx [...]
import {readFile} from 'node:fs/promises';
import {compile} from '@mdx-js/mdx';

let failed = false;
for (const file of process.argv.slice(2)) {
  try {
    await compile(await readFile(file, 'utf8'));
    console.log(`ok   ${file}`);
  } catch (error) {
    failed = true;
    console.log(`FAIL ${file}: ${error.message}`);
  }
}
process.exit(failed ? 1 : 0);
