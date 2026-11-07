// Applies the saved colour theme before the app loads, so a light-mode page
// does not flash dark first. Kept in sync with src/composables/theme.ts.
(function () {
  var mode = 'system'
  try { mode = localStorage.getItem('omnicd-theme') || 'system' } catch (e) { /* storage blocked */ }
  var dark = mode === 'dark' || (mode !== 'light' && !(window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches))
  document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light')
})()
