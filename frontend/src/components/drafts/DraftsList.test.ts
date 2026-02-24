import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '../../tests/svelte';
import DraftsList from './DraftsList.svelte';

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, text: async () => '' }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const draft = {
  id: 1,
  draftedAt: '2024-01-15T10:30:00Z',
  approvedAt: null,
  approvedBy: null,
  picks: [
    { id: 1, packId: 1, setName: 'Zendikar', productType: 'Draft Booster', marketPrice: 4.99 },
    { id: 2, packId: 2, setName: 'Alpha', productType: 'Set Booster', marketPrice: null },
  ],
};

describe('DraftsList', () => {
  it('shows empty message when no drafts', () => {
    const { container } = render(DraftsList, { props: { drafts: [], isAdmin: false } });
    expect(container.querySelector('.empty')).not.toBeNull();
  });

  it('renders draft date', () => {
    const { container } = render(DraftsList, { props: { drafts: [draft], isAdmin: false } });
    expect(container.querySelector('.draft-card__date')).not.toBeNull();
  });

  it('renders pick list items', () => {
    const { container } = render(DraftsList, { props: { drafts: [draft], isAdmin: false } });
    expect(container.querySelectorAll('.pick-item')).toHaveLength(2);
  });

  it('shows approved badge for approved draft', () => {
    const approvedDraft = { ...draft, approvedAt: '2024-01-16T10:00:00Z' };
    const { container } = render(DraftsList, { props: { drafts: [approvedDraft], isAdmin: true } });
    expect(container.querySelector('.badge--approved')).not.toBeNull();
  });

  it('shows no action buttons for approved draft even if admin', () => {
    const approvedDraft = { ...draft, approvedAt: '2024-01-16T10:00:00Z' };
    const { container } = render(DraftsList, { props: { drafts: [approvedDraft], isAdmin: true } });
    expect(container.querySelector('.btn-remove')).toBeNull();
    expect(container.querySelector('.btn-approve')).toBeNull();
  });

  it('shows remove and approve buttons for admin with unapproved draft', () => {
    const { container } = render(DraftsList, { props: { drafts: [draft], isAdmin: true } });
    expect(container.querySelector('.btn-remove')).not.toBeNull();
    expect(container.querySelector('.btn-approve')).not.toBeNull();
  });

  it('hides action buttons for non-admin', () => {
    const { container } = render(DraftsList, { props: { drafts: [draft], isAdmin: false } });
    expect(container.querySelector('.btn-remove')).toBeNull();
    expect(container.querySelector('.btn-approve')).toBeNull();
  });
});
