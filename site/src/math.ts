import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/sections.css";
import "./styles/math.css";
import "./styles/phone.css";

import { mountExplainers, mountMathHero, mountParity } from "./math/sections";
import { mountFooter } from "./sections/footer";
import { mountDiscreetField } from "./gl/slag";
import { wireReveals } from "./motion/reveal";
import { mountSection, plainSection, setViewportScale, wireAnchors } from "./motion/mount";
import { mountDeck } from "./motion/deck";
import { wireMode, wireScrollFades, wireTypeFloors } from "./motion/mode";
import { wireTopbar } from "./sections/topbar";
import { reducedMotion, startScrollObserver } from "./motion/scroll";

function main(): void {
  const w = window as unknown as { __shaderLog?: unknown[]; __flashLog?: unknown[] };
  w.__shaderLog ??= [];
  w.__flashLog ??= [];
  setViewportScale();
  wireMode();
  window.addEventListener("resize", setViewportScale);
  const app = document.getElementById("app")!;
  const main = document.createElement("main");
  if (reducedMotion()) main.classList.add("rm");
  // Page order: M0 hero (mounts on load), four pinned explainers (own geometry, §16), M5 (100 vh, held), footer.
  for (const p of [plainSection(mountMathHero()), ...mountExplainers(), mountSection(mountParity()), mountFooter()]) main.appendChild(p);
  app.appendChild(main);
  wireReveals(main);
  startScrollObserver();
  // a hash in the URL or a jump to an anchor lands on a fully mounted card (§0c)
  wireAnchors();
  wireTopbar();
  wireTypeFloors();
  wireScrollFades();
  // the discreet field runs behind the unpinned sections; the explainers have no furnace, so it stays on
  mountDeck(main, mountDiscreetField());
}

main();
