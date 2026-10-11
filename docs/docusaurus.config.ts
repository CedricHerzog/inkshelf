import path from 'node:path';
import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';
import repoLinks from './src/remark/repoLinks.mjs';

// Inkshelf's documentation: the learning chapters (learn/), from the big picture down to the
// algorithms with widgets that animate the hard ideas, and the reference (reference/), the
// precise and complete pages for changing the code, written to read well on GitHub too.

// repoUrl turns the code references into links into the repository. Empty: paths are shown as
// text.
const repoUrl = 'https://github.com/CedricHerzog/inkshelf/blob/main';

const config: Config = {
  title: 'Inkshelf, explained',
  tagline: 'How a pure-Go e-reader app draws books on e-ink',
  favicon: 'img/logo.svg',

  future: {
    v4: true,
  },

  // Published on GitHub Pages by .github/workflows/docs.yml; make docs serves it under the same
  // base, at http://localhost:3000/inkshelf/.
  url: 'https://cedricherzog.github.io',
  baseUrl: '/inkshelf/',
  organizationName: 'CedricHerzog',
  projectName: 'inkshelf',
  trailingSlash: false,

  onBrokenLinks: 'throw',
  onBrokenAnchors: 'throw',
  markdown: {
    mermaid: true,
    // .md files (the reference) are plain CommonMark, .mdx files (the chapters) are MDX.
    format: 'detect',
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },
  themes: ['@docusaurus/theme-mermaid'],

  customFields: {
    repoUrl,
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          path: 'learn',
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    [
      '@docusaurus/plugin-content-docs',
      {
        id: 'reference',
        path: 'reference',
        routeBasePath: 'reference',
        sidebarPath: './sidebars-reference.ts',
        beforeDefaultRemarkPlugins: [
          [repoLinks, {docsDir: path.resolve(__dirname), repoUrl}],
        ],
      },
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    metadata: [
      {
        name: 'description',
        content:
          'Inkshelf, an e-reader app for PocketBook in pure Go, explained: EPUB layout, fonts, ' +
          'e-ink refresh, dithering, and Komga, Calibre-Web and Mealie clients.',
      },
      {
        name: 'keywords',
        content:
          'PocketBook, InkPad, e-ink, e-reader, EPUB, CBZ, manga, comics, Komga, Calibre-Web, ' +
          'Mealie, Go, golang, framebuffer, typesetting',
      },
    ],
    colorMode: {
      respectPrefersColorScheme: true,
    },
    docs: {
      sidebar: {
        hideable: true,
      },
    },
    navbar: {
      title: 'Inkshelf, explained',
      logo: {
        alt: 'Inkshelf',
        src: 'img/logo.svg',
        srcDark: 'img/logo-dark.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'learn',
          position: 'left',
          label: 'Learn',
        },
        {
          type: 'docSidebar',
          sidebarId: 'reference',
          docsPluginId: 'reference',
          position: 'left',
          label: 'Reference',
        },
        {
          to: '/glossary',
          label: 'Glossary',
          position: 'left',
        },
        {
          href: 'https://github.com/CedricHerzog/inkshelf',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'light',
      links: [
        {
          label: 'Source code on GitHub',
          href: 'https://github.com/CedricHerzog/inkshelf',
        },
      ],
      copyright: 'Inkshelf: an e-reader app in pure Go, for Komga, Mealie and Calibre-Web.',
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.vsDark,
      additionalLanguages: ['go', 'bash', 'json', 'makefile'],
    },
    mermaid: {
      theme: {light: 'neutral', dark: 'dark'},
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
