import { test } from 'node:test';
import assert from 'node:assert/strict';
import { inspectStyles } from './style-inspection.mjs';

test('style inspection reports actual foreground and bounded ancestor backgrounds without a browser', () => {
  const previousDocument = globalThis.document;
  const previousComputedStyle = globalThis.getComputedStyle;
  const paint = { color: 'rgb(255, 220, 0)', backgroundColor: 'rgba(0, 0, 0, 0)', backgroundImage: 'none', fontSize: '22px', lineHeight: '28px', display: 'block', visibility: 'visible', opacity: '1', fill: 'rgb(255, 220, 0)', stroke: 'none' };
  const card = { tagName: 'DIV', id: '', getAttribute: () => 'card-accent', parentElement: null, style: { ...paint, backgroundColor: 'rgb(255, 220, 0)' } };
  const label = { tagName: 'P', id: '', getAttribute: () => 'kicker', parentElement: card, style: paint };
  globalThis.document = { querySelectorAll(selector) {
    if (selector === '[') throw new Error('invalid selector');
    return selector === '.missing' ? [] : Array(8).fill(label);
  } };
  globalThis.getComputedStyle = element => element.style;
  try {
    const [result, missing, invalid] = inspectStyles(['.bad .kicker', '.missing', '[']);
    assert.equal(result.match_count, 8);
    assert.equal(result.elements.length, 3);
    assert.equal(result.elements[0].computed.color, 'rgb(255, 220, 0)');
    assert.equal(result.elements[0].background_layers[0].color, 'rgb(255, 220, 0)');
    assert.deepEqual(missing.elements, []);
    assert.equal(invalid.error, 'invalid selector');
  } finally {
    globalThis.document = previousDocument;
    globalThis.getComputedStyle = previousComputedStyle;
  }
});
