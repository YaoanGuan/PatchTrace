import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeProjects, recentProjects } from './graphOrder.ts';

const project = (id, createdAt) => ({ id, created_at: createdAt });

test('recentProjects selects the newest projects first', () => {
    const list = [
        project('p1', '2026-11-01T00:00:00Z'),
        project('p2', '2026-11-02T00:00:00Z'),
        project('p3', '2026-11-03T00:00:00Z'),
        project('p4', '2026-11-04T00:00:00Z'),
        project('p5', '2026-11-05T00:00:00Z'),
        project('p6', '2026-11-06T00:00:00Z'),
    ];

    assert.deepEqual(recentProjects(list, 4).map(p => p.id), ['p6', 'p5', 'p4', 'p3']);
});

test('recentProjects does not mutate the server order', () => {
    const list = [
        project('old', '2026-11-01T00:00:00Z'),
        project('new', '2026-11-02T00:00:00Z'),
    ];

    recentProjects(list, 2);
    assert.deepEqual(list.map(p => p.id), ['old', 'new']);
});

test('recentProjects sorts undated projects after dated ones', () => {
    const list = [
        project('undated'),
        project('new', '2026-11-02T00:00:00Z'),
        project('invalid', 'not-a-timestamp'),
        project('old', '2026-11-01T00:00:00Z'),
    ];

    assert.deepEqual(recentProjects(list, 4).map(p => p.id), ['new', 'old', 'undated', 'invalid']);
});

test('recentProjects breaks timestamp ties deterministically by descending id', () => {
    const list = [
        project('a', '2026-11-01T00:00:00.000000000Z'),
        project('b', '2026-11-01T00:00:00.000000000Z'),
    ];

    assert.deepEqual(recentProjects(list, 2).map(p => p.id), ['b', 'a']);
});

test('recentProjects returns an empty list for a non-positive limit', () => {
    assert.deepEqual(recentProjects([project('p1', '2026-11-01T00:00:00Z')], 0), []);
});

test('normalizeProjects drops malformed elements instead of crashing the page', () => {
    const normalized = normalizeProjects([
        null,
        42,
        'project',
        {},
        { id: '' },
        { id: '   ' },
        { id: ' keep ', title: 'T', origin: 'O', status: 'active', bootstrap_enabled: true,
          created_at: '2026-11-01T00:00:00Z', reason: { worker: 'w1', trigger: 7 } },
    ]);

    assert.equal(normalized.length, 1);
    assert.equal(normalized[0].id, 'keep');
    assert.equal(normalized[0].title, 'T');
    assert.equal(normalized[0].origin, 'O');
    assert.equal(normalized[0].status, 'active');
    assert.equal(normalized[0].bootstrap_enabled, true);
    assert.equal(normalized[0].created_at, '2026-11-01T00:00:00Z');
    assert.equal(normalized[0].reason.worker, 'w1');
    assert.equal(normalized[0].reason.trigger, undefined);
});

test('normalizeProjects coerces non-string display fields to undefined', () => {
    const [p] = normalizeProjects([
        { id: 'p1', title: { nested: true }, origin: 5, status: [], created_at: 123, reason: 'worker' },
    ]);

    assert.equal(p.title, undefined);
    assert.equal(p.origin, undefined);
    assert.equal(p.status, undefined);
    assert.equal(p.created_at, undefined);
    assert.equal(p.reason, null);
});

