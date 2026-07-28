import { useLayoutEffect } from "react";
import { Nav } from "./components/Nav";
import { contentSpec } from "./content";
import { Hero } from "./sections/Hero";
import { Roles } from "./sections/Roles";
import { Philosophy } from "./sections/Philosophy";
import { Claims } from "./sections/Claims";
import { Lifecycle } from "./sections/Lifecycle";
import { Comments } from "./sections/Comments";
import { BuildOrder } from "./sections/BuildOrder";
import { CodeLinks } from "./sections/CodeLinks";
import { Cli } from "./sections/Cli";
import { Versions } from "./sections/Versions";
import { Compare } from "./sections/Compare";
import { Footer } from "./sections/Footer";

function HashAnchorSync() {
  useLayoutEffect(() => {
    const id = decodeURIComponent(window.location.hash.slice(1));
    if (!id) return;

    let cancelled = false;
    const align = () => {
      if (!cancelled) document.getElementById(id)?.scrollIntoView();
    };

    align();
    void document.fonts.ready.then(align);
    return () => {
      cancelled = true;
    };
  }, []);

  return null;
}

export default function App() {
  return (
    <>
      <HashAnchorSync />
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <Nav siteTitle={contentSpec.siteTitle} items={contentSpec.nav} />
      <main id="main">
        <Hero />
        {/* "Who runs what" sits second, before any command appears anywhere on
            the page: every section after it is written assuming the reader knows
            an agent operates this and a human reviews it. */}
        <Roles />
        <Philosophy />
        <Claims />
        <Lifecycle />
        <Comments />
        <BuildOrder />
        <CodeLinks />
        <Cli />
        <Versions />
        <Compare />
      </main>
      <Footer />
    </>
  );
}
