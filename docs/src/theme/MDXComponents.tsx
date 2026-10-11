// Makes the kit available in every chapter without an import: <Figure>, <Term>, <Quiz>…
// Chapter-specific widgets are still imported by the page that shows them.
import MDXComponents from '@theme-original/MDXComponents';
import * as kit from '@site/src/components/kit';

export default {
  ...MDXComponents,
  ...kit,
};
