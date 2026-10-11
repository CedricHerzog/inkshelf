// The domains of Inkshelf, bottom to top, and the rules that say which package may import
// which. It follows CLAUDE.md's domain map and docs/reference/architecture.md's layering rule; the real
// imports come from `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` (internal
// packages only, the "internal/" prefix dropped).

export type DomainId =
  | 'device'
  | 'graphics'
  | 'web'
  | 'typesetting'
  | 'ebook'
  | 'utilities'
  | 'servers'
  | 'shelf'
  | 'progress'
  | 'reading'
  | 'app'
  | 'cmd';

export type Domain = {
  id: DomainId;
  /** Height in the cake: a package may import from its own rank or a lower one. */
  rank: number;
  knows: string;
  packages: readonly string[];
};

// naturalorder and timing import nothing of Inkshelf's: they are utilities any layer may use,
// so they get the lowest rank although the map lists them among the domains.
export const domains: readonly Domain[] = [
  {
    id: 'utilities',
    rank: -1,
    knows:
      'Small helpers that import nothing of Inkshelf’s, so anyone may use them: naturalorder ("Tome 2" before "Tome 10") and timing (spans for the log and the execution trace, and profiles).',
    packages: ['naturalorder', 'timing'],
  },
  {
    id: 'device',
    rank: 0,
    knows:
      'The PocketBook without its SDK: framebuffer, e-ink refreshes, firmware messages, touch and keys, the event loop, Wi-Fi, and an emulated reader for the host.',
    packages: ['device/pocketbook'],
  },
  {
    id: 'graphics',
    rank: 1,
    knows:
      'Pixels: TrueType fonts and drawing text in gray; page images decoded, fitted, toned, dithered to 16 grays and packed into .pg4 files.',
    packages: ['graphics/fonts', 'graphics/pageimage'],
  },
  {
    id: 'web',
    rank: 2,
    knows:
      'The web formats books are written in: CSS, SVG, MathML, WOFF fonts, a book’s images, relative links and data: URIs.',
    packages: ['web/css', 'web/svgimage', 'web/mathml', 'web/webfont', 'web/webimage', 'web/uri'],
  },
  {
    id: 'typesetting',
    rank: 3,
    knows:
      'Text on pages: the document model (blocks of styled runs), line breaking, bidi, justification, floats, tables, pagination, hyphenation.',
    packages: ['typesetting/document', 'typesetting/layout', 'typesetting/hyphen'],
  },
  {
    id: 'ebook',
    rank: 4,
    knows:
      'Book files: an EPUB’s zip, package and navigation; its XHTML and CSS turned into documents; fixed-layout pages; the pages of a CBZ.',
    packages: ['ebook/epub/container', 'ebook/epub', 'ebook/epub/epubtest', 'ebook/fixedlayout', 'ebook/comicarchive'],
  },
  {
    id: 'servers',
    rank: 5,
    knows:
      'What the app needs from a server of books (the Server interface and its neutral model), and one package per kind of server.',
    packages: [
      'servers',
      'servers/httpclient',
      'servers/komga',
      'servers/komga/fakekomga',
      'servers/mealie',
      'servers/mealie/recipepages',
      'servers/mealie/fakemealie',
      'servers/calibreweb',
      'servers/calibreweb/fakecalibreweb',
      'servers/servertest',
      'servers/registry',
    ],
  },
  {
    id: 'shelf',
    rank: 6,
    knows: 'The books downloaded on the reader: downloading them, preparing them for the panel, book.json, covers.',
    packages: ['shelf'],
  },
  {
    id: 'progress',
    rank: 7,
    knows: 'Keeping the reading progress of the reader and of the server in agreement.',
    packages: ['progress'],
  },
  {
    id: 'reading',
    rank: 8,
    knows: 'The engine behind reading an EPUB whose text flows: positions, page turns across chapters, links, notes, contents.',
    packages: ['reading'],
  },
  {
    id: 'app',
    rank: 9,
    knows: 'The user interface: state, widgets, services, screens.',
    packages: [
      'app/widgets',
      'app/state',
      'app/session',
      'app/catalog',
      'app/downloadqueue',
      'app/covers',
      'app/openbook',
      'app/screens/list',
      'app/screens/startup',
      'app/screens/serieslist',
      'app/screens/booklist',
      'app/screens/reader',
      'app/screens/contents',
      'app/screens/coverview',
      'app',
    ],
  },
  {
    id: 'cmd',
    rank: 10,
    knows: 'The two programs: the one for the reader, and the emulator for the host.',
    packages: ['cmd/inkshelf', 'cmd/emulator'],
  },
];

/** The cake from the bottom up, the utilities apart. */
export const stack: readonly Domain[] = domains.filter((domain) => domain.id !== 'utilities');

/** The real imports between Inkshelf's packages, from go list. */
export const imports: Readonly<Record<string, readonly string[]>> = {
  'cmd/emulator': [
    'app',
    'device/pocketbook',
    'graphics/fonts',
    'servers',
    'servers/calibreweb/fakecalibreweb',
    'servers/httpclient',
    'servers/komga/fakekomga',
    'servers/mealie/fakemealie',
    'servers/registry',
    'timing',
  ],
  'cmd/inkshelf': ['app', 'device/pocketbook', 'servers/httpclient', 'servers/registry', 'timing'],
  app: [
    'app/catalog',
    'app/covers',
    'app/openbook',
    'app/screens/booklist',
    'app/screens/contents',
    'app/screens/coverview',
    'app/screens/reader',
    'app/screens/serieslist',
    'app/screens/startup',
    'app/session',
    'app/state',
    'app/widgets',
    'device/pocketbook',
    'graphics/fonts',
    'servers',
    'timing',
  ],
  'app/catalog': ['app/state', 'servers', 'shelf'],
  'app/covers': ['app/state', 'app/widgets', 'device/pocketbook', 'graphics/pageimage', 'servers', 'shelf', 'timing'],
  'app/downloadqueue': [
    'app/catalog',
    'app/covers',
    'app/state',
    'app/widgets',
    'device/pocketbook',
    'shelf',
    'timing',
  ],
  'app/openbook': [
    'app/catalog',
    'app/downloadqueue',
    'app/state',
    'app/widgets',
    'device/pocketbook',
    'progress',
    'reading',
    'servers',
    'shelf',
    'timing',
    'typesetting/layout',
  ],
  'app/screens/booklist': [
    'app/catalog',
    'app/covers',
    'app/downloadqueue',
    'app/openbook',
    'app/screens/list',
    'app/state',
    'app/widgets',
    'device/pocketbook',
    'progress',
    'servers',
    'shelf',
  ],
  'app/screens/contents': [
    'app/openbook',
    'app/screens/list',
    'app/state',
    'app/widgets',
    'device/pocketbook',
    'ebook/epub',
    'reading',
  ],
  'app/screens/coverview': ['app/state', 'app/widgets', 'device/pocketbook'],
  'app/screens/list': ['app/covers', 'app/state', 'app/widgets', 'device/pocketbook'],
  'app/screens/reader': [
    'app/catalog',
    'app/downloadqueue',
    'app/openbook',
    'app/state',
    'app/widgets',
    'device/pocketbook',
    'reading',
    'servers',
    'timing',
    'typesetting/layout',
  ],
  'app/screens/serieslist': [
    'app/catalog',
    'app/covers',
    'app/openbook',
    'app/screens/list',
    'app/state',
    'app/widgets',
    'device/pocketbook',
    'servers',
    'shelf',
  ],
  'app/screens/startup': ['app/state', 'app/widgets', 'device/pocketbook', 'servers'],
  'app/session': ['app/catalog', 'app/state', 'device/pocketbook', 'progress', 'servers', 'shelf'],
  'app/state': [
    'app/widgets',
    'device/pocketbook',
    'progress',
    'reading',
    'servers',
    'shelf',
    'timing',
    'typesetting/layout',
  ],
  'app/widgets': ['device/pocketbook', 'graphics/fonts', 'graphics/pageimage', 'servers'],
  'device/pocketbook': ['timing'],
  'ebook/comicarchive': ['naturalorder'],
  'ebook/epub': [
    'ebook/epub/container',
    'ebook/fixedlayout',
    'graphics/fonts',
    'timing',
    'typesetting/document',
    'web/css',
    'web/mathml',
    'web/svgimage',
    'web/uri',
  ],
  'ebook/epub/container': ['graphics/fonts', 'graphics/pageimage', 'web/css', 'web/uri', 'web/webfont', 'web/webimage'],
  'ebook/epub/epubtest': [],
  'ebook/fixedlayout': [
    'graphics/fonts',
    'graphics/pageimage',
    'typesetting/document',
    'typesetting/layout',
    'web/css',
    'web/svgimage',
    'web/uri',
    'web/webimage',
  ],
  'graphics/fonts': [],
  'graphics/pageimage': ['timing'],
  naturalorder: [],
  progress: ['servers', 'shelf'],
  reading: [
    'ebook/epub',
    'graphics/fonts',
    'shelf',
    'timing',
    'typesetting/document',
    'typesetting/layout',
    'web/webimage',
  ],
  servers: ['naturalorder'],
  'servers/calibreweb': ['servers', 'servers/httpclient'],
  'servers/calibreweb/fakecalibreweb': [],
  'servers/httpclient': ['timing'],
  'servers/komga': ['servers', 'servers/httpclient'],
  'servers/komga/fakekomga': [],
  'servers/mealie': ['servers', 'servers/httpclient', 'servers/mealie/recipepages'],
  'servers/mealie/fakemealie': [],
  'servers/mealie/recipepages': [],
  'servers/registry': ['servers', 'servers/calibreweb', 'servers/komga', 'servers/mealie'],
  'servers/servertest': ['servers'],
  shelf: [
    'ebook/comicarchive',
    'ebook/epub',
    'graphics/fonts',
    'graphics/pageimage',
    'naturalorder',
    'servers',
    'timing',
    'typesetting/layout',
    'web/svgimage',
    'web/uri',
  ],
  timing: [],
  'typesetting/document': ['graphics/fonts'],
  'typesetting/hyphen': [],
  'typesetting/layout': [
    'graphics/fonts',
    'typesetting/document',
    'typesetting/hyphen',
    'web/mathml',
    'web/svgimage',
    'web/webimage',
  ],
  'web/css': [],
  'web/mathml': [],
  'web/svgimage': ['graphics/pageimage', 'web/css', 'web/uri'],
  'web/uri': [],
  'web/webfont': [],
  'web/webimage': ['graphics/pageimage', 'web/svgimage', 'web/uri'],
};

export function domainOf(pkg: string): Domain {
  const found = domains.find((domain) => domain.packages.includes(pkg));
  if (!found) throw new Error(`layers: unknown package ${pkg}`);
  return found;
}

/** The domains a domain's packages import today, from the real imports. */
export function domainImports(id: DomainId): DomainId[] {
  const found = new Set<DomainId>();
  for (const pkg of domainById(id).packages) {
    for (const target of imports[pkg] ?? []) {
      const targetDomain = domainOf(target).id;
      if (targetDomain !== id) found.add(targetDomain);
    }
  }
  return domains.filter((domain) => found.has(domain.id)).map((domain) => domain.id);
}

export function domainById(id: DomainId): Domain {
  return domains.find((domain) => domain.id === id)!;
}

// The parts of the app, bottom to top (internal/app/doc.go): what a screen draws with, the
// state, the services over the state, the screens, and the app that dispatches to them.
const appParts: Record<string, {part: string; rank: number}> = {
  'app/widgets': {part: 'widgets', rank: 0},
  'app/state': {part: 'state', rank: 1},
  'app/session': {part: 'a service', rank: 2},
  'app/catalog': {part: 'a service', rank: 2},
  'app/downloadqueue': {part: 'a service', rank: 2},
  'app/covers': {part: 'a service', rank: 2},
  'app/openbook': {part: 'a service', rank: 2},
  'app/screens/list': {part: 'the shared list parts', rank: 3},
  app: {part: 'the app itself', rank: 4},
};

function appPart(pkg: string): {part: string; rank: number} | undefined {
  if (appParts[pkg]) return appParts[pkg];
  if (pkg.startsWith('app/screens/')) return {part: 'a screen', rank: 3};
  return undefined;
}

const specificServers = /^servers\/(komga|mealie|calibreweb)(\/|$)/;

/** pathTo returns a chain of real imports from `from` to `to`, or null if there is none. */
export function pathTo(from: string, to: string): string[] | null {
  const seen = new Set<string>([from]);
  const queue: string[][] = [[from]];
  while (queue.length > 0) {
    const path = queue.shift()!;
    const last = path[path.length - 1];
    for (const next of imports[last] ?? []) {
      if (next === to) return [...path, next];
      if (!seen.has(next)) {
        seen.add(next);
        queue.push([...path, next]);
      }
    }
  }
  return null;
}

export type Verdict = {
  allowed: boolean;
  /** The rule that decided, in one sentence. */
  reason: string;
  /** Whether the import exists in the code today. */
  real: boolean;
};

/** checkImport says whether package `from` may import package `to`, and why. */
export function checkImport(from: string, to: string): Verdict {
  const real = (imports[from] ?? []).includes(to);
  const fromDomain = domainOf(from);
  const toDomain = domainOf(to);
  const forbid = (reason: string): Verdict => ({allowed: false, reason, real});
  const allow = (reason: string): Verdict => ({allowed: true, reason, real});

  if (from === to) return forbid('A package does not import itself.');
  if (toDomain.id === 'cmd') {
    return forbid('Nothing imports a program: cmd is the top of the cake (and Go refuses to import a main package).');
  }
  if (to === 'servers/registry' && fromDomain.id !== 'cmd') {
    return forbid('Only the programs in cmd import servers/registry, the one place that knows every kind of server.');
  }
  if (to === 'servers/registry') {
    return allow('A program in cmd is the one place that picks the kind of server: servers/registry builds it from config.json.');
  }
  if (specificServers.test(to) && fromDomain.id !== 'cmd' && fromDomain.id !== 'servers') {
    return forbid(
      `${from} must not know a specific server: it talks to the servers.Server interface. Only cmd (through servers/registry) picks Komga, Mealie or Calibre-Web.`,
    );
  }
  if (fromDomain.rank < toDomain.rank) {
    const side = fromDomain.rank <= 4 && toDomain.id === 'servers' ? ' The reader side knows nothing of servers.' : '';
    return forbid(`${fromDomain.id} sits below ${toDomain.id}: a layer never imports one above it.${side}`);
  }
  const cycle = pathTo(to, from);
  if (cycle) {
    return forbid(`Go refuses import cycles, and ${to} already imports ${from} (${cycle.join(' → ')}).`);
  }
  if (fromDomain.id === 'app' && toDomain.id === 'app') {
    const fromPart = appPart(from)!;
    const toPart = appPart(to)!;
    if (fromPart.part === 'a screen' && toPart.part === 'a screen') {
      return forbid('Screens never import each other: a screen moves to another by its ScreenID, with state.ShowLocked.');
    }
    if (fromPart.rank < toPart.rank) {
      return forbid(`Inside the app, ${fromPart.part} sits below ${toPart.part}: widgets, then state, then services, then screens.`);
    }
    if (to === 'app/screens/list') {
      return allow('app/screens/list is not a screen: it is the header, rows and footer the list screens share, so they may import it.');
    }
  }
  if (fromDomain.id === toDomain.id) return allow(`Both are in ${fromDomain.id}: a domain may use its own packages.`);
  if (toDomain.id === 'utilities') return allow(`${to} imports nothing of Inkshelf's: any layer may use it.`);
  return allow(`${toDomain.id} is below ${fromDomain.id}: importing downwards is always fine.`);
}
