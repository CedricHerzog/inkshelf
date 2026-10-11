import path from 'node:path';
import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';
import repoLinks from './src/remark/repoLinks.mjs';

// Inkshelf's documentation: the learning chapters (learn/), from the big picture down to the
// algorithms with widgets that animate the hard ideas, and the reference (reference/), the
// precise and complete pages for changing the code, written to read well on GitHub too.

// repoUrl turns the code references into links once the repository is published, for example
// 'https://github.com/<user>/inkshelf/blob/main'. Empty: paths are shown as text.
const repoUrl = '';

const config: Config = {
  title: 'Inkshelf, explained',
  tagline: 'How a pure-Go e-reader app draws books on e-ink',
  favicon: 'img/logo.svg',

  future: {
    v4: true,
  },

  url: 'http://localhost',
  baseUrl: '/',

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
      ],
    },
    footer: {
      style: 'light',
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
