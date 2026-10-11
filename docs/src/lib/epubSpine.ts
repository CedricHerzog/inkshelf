// How Inkshelf turns a package document's manifest and spine into chapters. A TypeScript copy
// of readSpine and firstSupported (internal/ebook/epub/container/package_document.go).

export type ManifestItem = {
  id: string;
  href: string;
  mediaType: string;
  properties?: string;
  fallback?: string;
};

export type ItemRef = {idref: string; linear?: 'no'};

/** documentTypes and imageTypes: the media types a chapter can be shown from. */
export const documentTypes = ['application/xhtml+xml', 'text/html'];
export const imageTypes = [
  'image/gif', 'image/jpeg', 'image/jpg', 'image/jxl', 'image/png', 'image/svg+xml', 'image/webp',
];

/** A hop of a fallback chain: the item looked at, and whether it can be shown. */
export type Hop = {id: string; supported: boolean};

export type SpineItem = {
  idref: string;
  linear: boolean;
  /** The item shown, the first of the fallback chain that can be: '' if none can. */
  content: string;
  /** The items looked at, in order (firstSupported's walk). */
  hops: Hop[];
};

function isContent(item: ManifestItem): boolean {
  const properties = (item.properties ?? '').split(/\s+/);
  if (item.fallback && properties.includes('scripted')) return false; // its fallback is shown
  return documentTypes.includes(item.mediaType) || imageTypes.includes(item.mediaType);
}

/**
 * firstSupported follows the fallback chain from id to the first item that can be shown,
 * stopping at the first item seen twice.
 */
export function firstSupported(
  manifest: readonly ManifestItem[],
  id: string,
): {item: ManifestItem | null; hops: Hop[]} {
  const seen = new Set<string>();
  const hops: Hop[] = [];
  let current: string | undefined = id;
  while (current !== undefined && !seen.has(current)) {
    const item = manifest.find((candidate) => candidate.id === current);
    if (!item) break;
    const supported = isContent(item);
    hops.push({id: item.id, supported});
    if (supported) return {item, hops};
    seen.add(current);
    current = item.fallback;
  }
  return {item: null, hops};
}

/** readSpine returns every spine item, and the chapters: the linear ones. */
export function readSpine(
  manifest: readonly ManifestItem[],
  spine: readonly ItemRef[],
): {spine: SpineItem[]; chapters: SpineItem[]} {
  const items: SpineItem[] = [];
  const chapters: SpineItem[] = [];
  for (const itemRef of spine) {
    const item = manifest.find((candidate) => candidate.id === itemRef.idref);
    if (!item) continue; // Inkshelf refuses the book: "spine item is not in the manifest"
    const {item: content, hops} = firstSupported(manifest, item.id);
    let shown = content?.href ?? '';
    if (!content && (item.mediaType === '' || /xml|html/.test(item.mediaType))) {
      shown = item.href; // no fallback, and a media type books get wrong: read as XHTML
    }
    const spineItem = {idref: itemRef.idref, linear: itemRef.linear !== 'no', content: shown, hops};
    items.push(spineItem);
    if (spineItem.linear) chapters.push(spineItem);
  }
  return {spine: items, chapters};
}
