// A remark plugin for the reference pages, which are written to be read on GitHub too: their
// relative links into the code (../../internal/…, ../../Makefile) leave the site. On the site,
// such a link points into the repository when customFields.repoUrl is set, and becomes plain
// text otherwise, as <CodeRef> does.

import path from 'node:path';

export default function repoLinks({docsDir, repoUrl}) {
  const repoRoot = path.dirname(docsDir);
  return (tree, file) => {
    const fileDir = path.dirname(file.path);
    rewrite(tree);

    function rewrite(node) {
      if (!node.children) return;
      node.children = node.children.flatMap((child) => {
        if (child.type === 'link' && isRelative(child.url)) {
          const target = path.resolve(fileDir, child.url.split('#')[0]);
          if (!target.startsWith(docsDir + path.sep)) {
            const repoPath = path.relative(repoRoot, target);
            if (repoUrl) {
              child.url = `${repoUrl}/${repoPath}`;
            } else {
              rewrite(child);
              return child.children;
            }
          }
        }
        rewrite(child);
        return [child];
      });
    }
  };
}

function isRelative(url) {
  return !/^[a-z]+:/i.test(url) && !url.startsWith('/') && !url.startsWith('#');
}
