import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// One path through the site, from the easiest chapter to the hardest: each chapter only uses
// ideas taught before it.
const sidebars: SidebarsConfig = {
  learn: [
    {
      type: 'category',
      label: 'Start here',
      collapsed: false,
      items: ['intro', 'glossary'],
    },
    {
      type: 'category',
      label: '1. The big picture',
      collapsed: false,
      items: ['big-picture/journey', 'big-picture/layers'],
    },
    {
      type: 'category',
      label: '2. The screen',
      items: [
        'screen/eink',
        'screen/grays',
        'screen/no-sdk',
        'screen/firmware',
        'screen/event-loop',
        'screen/emulator',
      ],
    },
    {
      type: 'category',
      label: '3. Books',
      items: ['books/epub', 'books/css', 'books/fonts', 'books/lines', 'books/pages'],
    },
    {
      type: 'category',
      label: '4. The app',
      items: ['app/tap', 'app/locks', 'app/servers', 'app/downloads', 'app/sync'],
    },
    {
      type: 'category',
      label: 'Appendix',
      items: ['appendix/code-tour', 'appendix/where-to-look', 'appendix/contributing'],
    },
  ],
};

export default sidebars;
