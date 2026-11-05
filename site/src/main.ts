import "./styles/tokens.css";
import "./styles/base.css";
import "./styles/sections.css";
import "./styles/dive.css";
import "./styles/phone.css";

import { mountHero } from "./sections/hero";
import { mountPains } from "./sections/pains";
import { mountGates } from "./sections/gates";
import { mountTests } from "./sections/tests";
import { mountHelps } from "./sections/helps";
import { mountSetup } from "./sections/setup";
import { mountDive } from "./dive/dive";
import { mountNumbers } from "./sections/numbers";
import { mountMath } from "./sections/math";
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

  // Page order (§12): P0 hero, P1 pains, P2 how it helps, P3 gates beat reasoning, P4 the tests (its CTA types the
  // bridge into K1), K1→K4 (pinned, 400 vh of scrub), S9, M teaser, the recommended setup (§08), footer.
  // Every card is 100 vh in the sticky deck (§0c).
  const dive = mountDive();
  const hero = mountHero();
  const heroWrap = plainSection(hero.el);
  hero.attachField(heroWrap);
  const parts: HTMLElement[] = [
    heroWrap,
    mountSection(mountPains()),
    mountSection(mountHelps()),
    mountSection(mountGates()),
    mountSection(mountTests()),
    dive.el,
    mountSection(mountNumbers()),
    mountSection(mountMath()),
    mountSection(mountSetup()),
    mountFooter(),
  ];
  for (const p of parts) main.appendChild(p);
  app.appendChild(main);

  wireReveals(main);
  startScrollObserver();
  // a hash in the URL or a jump to an anchor lands on a fully mounted card (§0c)
  wireAnchors();
  wireTopbar();
  wireTypeFloors();
  wireScrollFades();
  // the discreet slag field behind every unpinned section, never inside the K dive (§0c); the deck hands it to
  // the active card so it is clipped to that card
  mountDeck(main, mountDiscreetField());
}

main();
