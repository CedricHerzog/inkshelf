// The glossary page: every term of src/data/glossary.ts, grouped, each with an anchor that
// <Term> links to.
import React from 'react';
import Link from '@docusaurus/Link';
import useBrokenLinks from '@docusaurus/useBrokenLinks';
import {glossary, type GlossaryEntry} from '@site/src/data/glossary';

const groups: GlossaryEntry['group'][] = ['Screen', 'Device', 'Images', 'Books', 'Layout', 'App', 'Servers'];

export default function GlossaryList() {
  // The anchors are made here, not by Markdown headings: tell the link checker about them.
  const brokenLinks = useBrokenLinks();
  glossary.forEach((entry) => brokenLinks.collectAnchor(entry.id));
  return (
    <>
      {groups.map((group) => (
        <section key={group}>
          <h2>{group}</h2>
          <dl>
            {glossary
              .filter((entry) => entry.group === group)
              .map((entry) => (
                <React.Fragment key={entry.id}>
                  <dt id={entry.id} style={{scrollMarginTop: '5rem', marginTop: '1rem'}}>
                    <strong>{entry.term}</strong>
                  </dt>
                  <dd style={{marginLeft: 0}}>
                    {entry.long ?? entry.short}
                    {entry.chapter && (
                      <>
                        {' '}
                        <Link to={entry.chapter}>Learn more →</Link>
                      </>
                    )}
                  </dd>
                </React.Fragment>
              ))}
          </dl>
        </section>
      ))}
    </>
  );
}
