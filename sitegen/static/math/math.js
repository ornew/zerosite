// Renders the formulas of a page with KaTeX. The Markdown step writes each formula as
// <span class="math math-inline"> or <div class="math math-display"> holding its TeX source as text.
// The build links this script, with katex.min.css, only from pages that contain a formula. The
// KaTeX build is published beside it, so the page's Content-Security-Policy needs no remote origin.
// Only the elements that the Markdown step marked are rendered; a formula that KaTeX rejects keeps
// its source and logs the error.
(function () {
  const KATEX = new URL("katex.min.js", document.currentScript.src).href;

  // loadKaTeX loads the vendored build, which defines the global katex.
  function loadKaTeX() {
    return new Promise((resolve, reject) => {
      if (globalThis.katex) return resolve(globalThis.katex);
      const s = document.createElement("script");
      s.src = KATEX;
      s.onload = () => resolve(globalThis.katex);
      s.onerror = () => reject(new Error("cannot load " + KATEX));
      document.head.append(s);
    });
  }

  async function main() {
    const elements = [...document.querySelectorAll("span.math-inline, div.math-display")];
    if (elements.length === 0) return;
    const katex = await loadKaTeX();
    for (const el of elements) {
      const tex = el.textContent;
      try {
        katex.render(tex, el, {
          displayMode: el.classList.contains("math-display"),
          throwOnError: true,
        });
      } catch (err) {
        // katex.render empties the element before it fails, so the source is put back.
        el.textContent = tex;
        console.error("Math formula failed:", err);
      }
    }
  }

  main().catch((err) => console.error(err));
})();
