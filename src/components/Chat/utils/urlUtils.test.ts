import { toRelativeUrl, extractPathFromUrl, getTabLabel } from './urlUtils';

describe('toRelativeUrl', () => {
  it('keeps relative dashboard URLs and appends kiosk mode', () => {
    expect(toRelativeUrl('/d/abc123/my-dashboard')).toBe('/d/abc123/my-dashboard?kiosk');
  });

  it('keeps relative explore URLs', () => {
    expect(toRelativeUrl('/explore?left={"datasource":"loki"}')).toBe('/explore?left={"datasource":"loki"}&kiosk');
  });

  it('does not duplicate kiosk parameter', () => {
    expect(toRelativeUrl('/d/abc/x?kiosk')).toBe('/d/abc/x?kiosk');
  });

  it('respects kioskModeEnabled = false', () => {
    expect(toRelativeUrl('/d/abc/x', false)).toBe('/d/abc/x');
  });

  it('strips the host from absolute same-site dashboard URLs', () => {
    expect(toRelativeUrl('https://grafana.example.com/d/abc/x')).toBe('/d/abc/x?kiosk');
  });

  it('rejects absolute URLs without a path', () => {
    // https://evil.com previously passed through unchanged and became an
    // external iframe inside the Grafana panel.
    expect(toRelativeUrl('https://evil.com')).toBe('');
    expect(toRelativeUrl('http://169.254.169.254')).toBe('');
  });

  it('rejects absolute URLs pointing at non-Grafana paths', () => {
    expect(toRelativeUrl('https://grafana.example.com/admin/users')).toBe('');
    expect(toRelativeUrl('https://evil.com/d/abc/x')).toBe('/d/abc/x?kiosk'); // path kept, host dropped
  });

  it('rejects non-http schemes and javascript: URLs', () => {
    expect(toRelativeUrl('javascript:alert(1)')).toBe('');
    expect(toRelativeUrl('data:text/html,<script>alert(1)</script>')).toBe('');
    expect(toRelativeUrl('file:///etc/passwd')).toBe('');
  });

  it('rejects protocol-relative and relative non-Grafana paths', () => {
    expect(toRelativeUrl('//evil.com/d/x')).toBe('');
    expect(toRelativeUrl('/api/admin/settings')).toBe('');
    expect(toRelativeUrl('/dashboard/new')).toBe('');
  });

  it('rejects empty input', () => {
    expect(toRelativeUrl('')).toBe('');
  });
});

describe('extractPathFromUrl', () => {
  it('extracts path from absolute URL', () => {
    expect(extractPathFromUrl('https://grafana.example.com/d/abc/x')).toBe('/d/abc/x');
  });

  it('returns relative URL unchanged', () => {
    expect(extractPathFromUrl('/explore?left={}')).toBe('/explore?left={}');
  });
});

describe('getTabLabel', () => {
  it('truncates long titles', () => {
    expect(getTabLabel({ url: '/d/x', type: 'dashboard', title: 'a'.repeat(30) }, 0)).toBe('a'.repeat(20) + '...');
  });

  it('falls back to type labels', () => {
    expect(getTabLabel({ url: '/explore', type: 'explore' }, 0)).toBe('Explore');
    expect(getTabLabel({ url: '/d/abcdef123456/x', type: 'dashboard', uid: 'abcdef123456' }, 0)).toBe(
      'Dashboard abcdef12'
    );
    expect(getTabLabel({ url: '/d/x', type: 'dashboard' }, 3)).toBe('Page 4');
  });
});
