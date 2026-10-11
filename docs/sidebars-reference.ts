import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// The reference: what you read before changing the code. The learning chapters teach the same
// domains from scratch; each one links to its reference page.
const sidebars: SidebarsConfig = {
  reference: [
    'usage',
    'architecture',
    {
      type: 'category',
      label: 'The reader',
      collapsed: false,
      items: ['eink-and-firmware', 'image-pipeline', 'emulator'],
    },
    {
      type: 'category',
      label: 'Books',
      collapsed: false,
      items: ['epub-conformance', 'epub-to-page', 'document-model', 'pagination'],
    },
    {
      type: 'category',
      label: 'The app',
      collapsed: false,
      items: ['app-threading', 'servers', 'progress-sync', 'on-disk-formats', 'profiling'],
    },
  ],
};

export default sidebars;
