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
