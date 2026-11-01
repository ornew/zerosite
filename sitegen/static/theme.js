(() => {
  try {
    const choice = localStorage.getItem('zerosite-theme');
    if (choice === 'light' || choice === 'dark') document.documentElement.dataset.theme = choice;
  } catch { /* Rendering still follows system colors if storage is unavailable. */ }
})();
