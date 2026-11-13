(function () {
  var root = document.documentElement;

  // Theme toggle, as on calendar.uug.ai.
  var toggle = document.getElementById('theme-toggle');
  function syncToggle() {
    var dark = root.getAttribute('data-theme') === 'dark';
    var label = 'Switch to ' + (dark ? 'light' : 'dark') + ' mode';
    toggle.setAttribute('aria-pressed', String(dark));
    toggle.setAttribute('aria-label', label);
    toggle.title = label;
  }
  toggle.addEventListener('click', function () {
    var next = root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
    root.setAttribute('data-theme', next);
    try { localStorage.setItem('inspect-theme', next); } catch (e) {}
    syncToggle();
  });
  syncToggle();

  // Respect reduced motion for the demo video.
  var video = document.querySelector('.demo-frame video');
  if (video && window.matchMedia && matchMedia('(prefers-reduced-motion: reduce)').matches) {
    video.removeAttribute('autoplay');
    video.pause();
    video.controls = true;
  }

  // Live demo: the real library on this page, with submit answered locally.
  var inspect = null;
  function start() {
    if (inspect || !window.UugInspect) return inspect;
    inspect = window.UugInspect.createInspect({
      accentColor: getComputedStyle(root).getPropertyValue('--primary').trim() || '#2713ff',
      getContext: function () { return { page: 'inspect.uug.ai', mode: 'demo' }; },
      // Demo mode: "Write it for me" answers locally with an example draft.
      onAssist: function (req) {
        var notes = (req.description || req.title || '').trim();
        return new Promise(function (resolve) {
          setTimeout(function () {
            resolve({
              title: 'Landing page: ' + (notes.slice(0, 60) || 'report card illustration is misaligned'),
              description: [
                '## Summary',
                (notes || 'Something on the landing page looks wrong.') + ' (' + req.images.length + ' screenshot(s) attached.)',
                '',
                '## Steps to reproduce',
                '1. Open ' + (req.context.url || 'the page') + '.',
                '2. Look at the area marked in screenshot 1.',
                '',
                '## Expected behaviour',
                'The section renders as designed.',
                '',
                '## Actual behaviour',
                'See the annotations in the screenshots.',
                '',
                '## Acceptance criteria',
                '- [ ] The marked area matches the design at desktop and phone widths.',
                '- [ ] Light and dark themes both look right.',
                '',
                '## Test cases',
                '1. Given a 1440 px wide window, when the page loads, then the section matches the design.',
                '2. Given a 390 px wide window, when the page loads, then nothing overflows horizontally.',
              ].join('\n'),
            });
          }, 1200);
        });
      },
      onSubmit: function () {
        // Demo mode: nothing leaves the browser.
        return new Promise(function (resolve) { setTimeout(function () { resolve({}); }, 700); });
      },
    });
    return inspect;
  }
  window.addEventListener('load', start);
  document.querySelectorAll('[data-try]').forEach(function (button) {
    button.addEventListener('click', function () {
      var i = start();
      if (i) i.open();
    });
  });
})();
