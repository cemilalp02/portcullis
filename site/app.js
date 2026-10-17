// Progressive enhancement only: the page is complete without this file.

// The hero file: show it first as you see it, then as your agent reads it,
// and let the reader switch between the two.
const pane = document.querySelector(".pane");
if (pane) {
  const buttons = pane.querySelectorAll(".toggle button");
  const show = (view) => {
    pane.dataset.view = view;
    buttons.forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.set === view)));
  };
  let touched = false;
  buttons.forEach((b) => b.addEventListener("click", () => { touched = true; show(b.dataset.set); }));
  if (!matchMedia("(prefers-reduced-motion: reduce)").matches) {
    show("you");
    setTimeout(() => { if (!touched) show("agent"); }, 1600);
  }
}

// Copy buttons next to commands.
document.querySelectorAll(".copy").forEach((button) => {
  button.addEventListener("click", async () => {
    const code = button.parentElement.querySelector("code").textContent.replace(/^\$\s*/, "");
    try {
      await navigator.clipboard.writeText(code);
      button.textContent = "Copied";
      button.dataset.done = "";
    } catch {
      button.textContent = "Select and copy";
    }
    setTimeout(() => {
      button.textContent = "Copy";
      delete button.dataset.done;
    }, 2000);
  });
});
