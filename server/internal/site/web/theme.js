// Runs in <head> so the page never flashes the wrong theme. Same key style as calendar.
(function () {
  var theme = 'light';
  try {
    theme = localStorage.getItem('inspect-theme') ||
      (window.matchMedia && matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  } catch (e) {}
  document.documentElement.setAttribute('data-theme', theme);
})();
