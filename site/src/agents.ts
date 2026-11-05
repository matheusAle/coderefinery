import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/sections.css";
import "./styles/agents.css";
import "./styles/phone.css";

import { attachAgentsField, mountAgentsHero, mountEverywhere, mountLoop, mountReliable, mountVerdicts } from "./agents/sections";
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
  // Page order: A0 (mounts on load), A1, A2, A3, A4, footer; each 100 vh, held for 60 vh (§0c).
  const a0 = plainSection(mountAgentsHero());
  attachAgentsField(a0);
  for (const p of [a0, mountSection(mountLoop()), mountSection(mountVerdicts()), mountSection(mountReliable()), mountSection(mountEverywhere()), mountFooter()]) {
    main.appendChild(p);
  }
  app.appendChild(main);
  wireReveals(main);
  startScrollObserver();
  // a hash in the URL or a jump to an anchor lands on a fully mounted card (§0c)
  wireAnchors();
  wireTopbar();
  wireTypeFloors();
  wireScrollFades();
  mountDeck(main, mountDiscreetField());
}

main();
