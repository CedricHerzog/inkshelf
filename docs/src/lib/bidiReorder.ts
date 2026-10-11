// Rule L2 of the Unicode bidirectional algorithm, as reorder in
// internal/typesetting/layout/bidi.go does it: from the highest level down to the lowest odd one,
// reverse every run of items at that level or higher. It records the order after each level.

export type ReorderStep<Item> = {
  /** The level of this pass: runs at this level or higher were reversed. */
  level: number;
  /** The runs reversed, as [start, end) in the order before the pass. */
  runs: [number, number][];
  order: Item[];
};

export function reorderSteps<Item>(items: readonly Item[], level: (item: Item) => number): ReorderStep<Item>[] {
  let highest = 0;
  let lowestOdd = 127;
  for (const item of items) {
    highest = Math.max(highest, level(item));
    lowestOdd = Math.min(lowestOdd, level(item) | 1);
  }
  const order = [...items];
  const steps: ReorderStep<Item>[] = [];
  for (let current = highest; current >= lowestOdd; current--) {
    const runs: [number, number][] = [];
    for (let start = 0; start < order.length; ) {
      if (level(order[start]) < current) {
        start++;
        continue;
      }
      let end = start;
      while (end < order.length && level(order[end]) >= current) end++;
      runs.push([start, end]);
      const reversed = order.slice(start, end).reverse();
      order.splice(start, end - start, ...reversed);
      start = end;
    }
    steps.push({level: current, runs, order: [...order]});
  }
  return steps;
}
