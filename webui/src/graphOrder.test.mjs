import test from 'node:test';
import assert from 'node:assert/strict';
import { recentProjects } from './graphOrder.ts';

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
