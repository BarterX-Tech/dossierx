const toggle = document.querySelector('.theme');

function syncThemeToggle() {
  toggle.setAttribute('aria-checked', String(document.documentElement.dataset.theme === 'dark'));
}

syncThemeToggle();
toggle.addEventListener('click', () => {
  const next = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
  document.documentElement.dataset.theme = next;
  try {
    localStorage.setItem('dossierx-theme', next);
  } catch (error) {
    // The toggle still works when browser storage is unavailable.
  }
  syncThemeToggle();
});
